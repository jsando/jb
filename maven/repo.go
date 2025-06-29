package maven

import (
	"encoding/xml"
	"fmt"
	"golang.org/x/net/html/charset"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const MAVEN_CENTRAL_URL = "https://repo.maven.apache.org/maven2/"

type LocalRepository struct {
	baseDir string
	remotes []string
	poms    map[string]*POM
}

var mavenVarPattern = regexp.MustCompile(`\$\{([a-zA-Z0-9._-]+)\}`)

func OpenLocalRepository() *LocalRepository {
	return &LocalRepository{
		baseDir: "~/.jb/repository",
		remotes: []string{MAVEN_CENTRAL_URL},
		poms:    make(map[string]*POM),
	}
}

func GAV(groupID, artifactID, version string) string {
	return groupID + ":" + artifactID + ":" + version
}

func jarFile(artifactID, version string) string {
	return fmt.Sprintf("%s-%s.jar",
		artifactID,
		version,
	)
}

func pomFile(artifactID, version string) string {
	return fmt.Sprintf("%s-%s.pom",
		artifactID,
		version,
	)
}

func (jc *LocalRepository) artifactDir(groupID, artifactID, version string) string {
	groupIDWithSlashes := strings.ReplaceAll(groupID, ".", "/")
	relPath := filepath.Join(strings.Split(groupIDWithSlashes, "/")...)
	relPath = filepath.Join(relPath, artifactID, version)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(strings.ReplaceAll(jc.baseDir, "~", homeDir), relPath)
}

func (c *LocalRepository) GetPOM(groupID, artifactID, version string) (*POM, error) {
	if groupID == "" || artifactID == "" || version == "" {
		return nil, fmt.Errorf("invalid maven coordinates %s:%s:%s", groupID, artifactID, version)
	}
	gav := GAV(groupID, artifactID, version)
	pom, found := c.poms[gav]
	if found {
		return pom, nil
	}
	pom = &POM{}
	pomFile := pomFile(artifactID, version)
	path, err := c.getFile(groupID, artifactID, version, pomFile)
	if err != nil {
		return pom, err
	}
	xmlFile, err := os.Open(path)
	if err != nil {
		return pom, err
	}
	defer xmlFile.Close()
	decoder := xml.NewDecoder(xmlFile)
	decoder.CharsetReader = charset.NewReaderLabel
	err = decoder.Decode(&pom)
	if err == nil {
		c.poms[gav] = pom
	}

	if pom.DependencyManagement == nil {
		pom.DependencyManagement = &DependencyManagement{
			Dependencies: make([]Dependency, 0),
		}
	}
	if pom.Dependencies == nil {
		pom.Dependencies = make([]Dependency, 0)
	}

	if pom.Parent != nil {
		err = c.expandParentProperties(pom)
		if err != nil {
			return nil, err
		}
	}
	pom.SetProperty("project.version", pom.Version)

	for i := range pom.DependencyManagement.Dependencies {
		pom.DependencyManagement.Dependencies[i].Version = pom.Expand(pom.DependencyManagement.Dependencies[i].Version)
		if pom.DependencyManagement.Dependencies[i].Scope == "import" && pom.DependencyManagement.Dependencies[i].Type == "pom" {
			// nested include dependency management from this other pom
			dep := pom.DependencyManagement.Dependencies[i]
			includePOM, err := c.GetPOM(dep.GroupID, dep.ArtifactID, dep.Version)
			if err != nil {
				return nil, err
			}
			mergeParentDeps(pom.DependencyManagement, includePOM.DependencyManagement)
		}
	}
	for i := range pom.Dependencies {
		if pom.Dependencies[i].Version == "" {
			dmDep := pom.findDependency(pom.Dependencies[i].GroupID, pom.Dependencies[i].ArtifactID)
			if dmDep != nil {
				pom.Dependencies[i].Version = dmDep.Version
			}
		}
	}
	for i := range pom.Dependencies {
		if pom.Dependencies[i].Version != "" {
			pom.Dependencies[i].Version = pom.Expand(pom.Dependencies[i].Version)
		}
	}

	// todo sanity check, make sure POM has version and all

	// Pretty print the POM to the terminal
	//dump(pom)

	return pom, err
}

func (c *LocalRepository) expandParentProperties(pom *POM) error {
	parent, err := c.GetPOM(pom.Parent.GroupID, pom.Parent.ArtifactID, pom.Parent.Version)
	if err != nil {
		return err
	}

	if pom.GroupID == "" {
		pom.GroupID = parent.GroupID
	}
	if pom.Version == "" {
		pom.Version = parent.Version
	}
	mergeParentDeps(pom.DependencyManagement, parent.DependencyManagement)
	mergeParentProperties(pom, parent)
	pom.SetProperty("project.parent.version", parent.Version)
	return nil
}

// ResolveMavenFields replaces ${...} placeholders in a Maven field string with their corresponding values from properties map.
// If a placeholder cannot be resolved, it remains unchanged.
func ResolveMavenFields(input string, properties map[string]string) string {

	// Replace function to resolve each placeholder
	result := mavenVarPattern.ReplaceAllStringFunc(input, func(match string) string {
		// Extract the key within ${...}
		key := mavenVarPattern.FindStringSubmatch(match)[1]
		// Look up the key in the properties map
		if value, exists := properties[key]; exists {
			return value
		}
		// Return the original placeholder if not found
		return match
	})
	return result
}

func (p POM) findDependency(groupID, artifactID string) *Dependency {
	if p.DependencyManagement != nil && p.DependencyManagement.Dependencies != nil {
		for _, dep := range p.DependencyManagement.Dependencies {
			if dep.GroupID == groupID && dep.ArtifactID == artifactID {
				return &dep
			}
		}
	}
	return nil
}

func mergeParentProperties(child, parent *POM) {
	if child.Properties == nil {
		child.Properties = &Properties{
			Properties: make([]Property, 0),
		}
	}
	oldProps := child.Properties.Properties
	child.Properties.Properties = make([]Property, 0)
	if parent.Properties != nil && parent.Properties.Properties != nil {
		child.Properties.Properties = append(child.Properties.Properties, parent.Properties.Properties...)
	}
	for _, prop := range oldProps {
		child.SetProperty(prop.XMLName.Local, prop.Value)
	}
}

func mergeParentDeps(child *DependencyManagement, parent *DependencyManagement) {
	if child.Dependencies == nil {
		child.Dependencies = make([]Dependency, 0)
	}
	if parent == nil {
		return
	}
	if parent.Dependencies != nil {
		for _, dep := range parent.Dependencies {
			found := false
			for _, childDep := range child.Dependencies {
				if dep.GroupID == childDep.GroupID && dep.ArtifactID == childDep.ArtifactID {
					found = true
					break
				}
			}
			if !found {
				child.Dependencies = append(child.Dependencies, dep)
			}
		}
	}
}

func (c *LocalRepository) GetJAR(groupID, artifactID, version string) (string, error) {
	return c.getFile(groupID, artifactID, version, jarFile(artifactID, version))
}

func (c *LocalRepository) getFile(groupID, artifactID, version, file string) (string, error) {
	artifactPath := filepath.Join(c.artifactDir(groupID, artifactID, version), file)
	if fileExists(artifactPath) {
		return artifactPath, nil
	}
	err := os.MkdirAll(filepath.Dir(artifactPath), 0755)
	//fmt.Printf("mkdir -p %s\n", filepath.Dir(artifactPath))
	if err != nil {
		return "", err
	}
	outFile, err := os.OpenFile(artifactPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return artifactPath, err
	}
	defer outFile.Close()
	for _, remote := range c.remotes {
		err = fetchFromRemote(remote, groupID, artifactID, version, file, outFile)
		if err == nil {
			break
		}
		fmt.Printf("error fetching from maven %s: %s\n", remote, err.Error())
	}
	// not found ... delete empty file and return error
	if err != nil {
		outFile.Close()
		os.Remove(artifactPath)
		return artifactPath, fmt.Errorf("failed to fetch %s from any remote", artifactPath)
	}
	return artifactPath, err
}

func (c *LocalRepository) InstallPackage(groupID, artifactID, version, jarPath, pomPath string) error {
	pomFileName := fmt.Sprintf("%s-%s.pom", artifactID, version)
	jarFileName := fmt.Sprintf("%s-%s.jar", artifactID, version)
	artifactDir := c.artifactDir(groupID, artifactID, version)
	preRelease := strings.Contains(version, "-")

	// Create the maven directory if it doesn't exist
	err := os.MkdirAll(artifactDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create maven directory: %w", err)
	}

	// Copy POM file
	destPomPath := filepath.Join(artifactDir, pomFileName)
	if fileExists(destPomPath) && !preRelease {
		return fmt.Errorf("POM file already exists at %s and version is not a pre-release", destPomPath)
	}
	err = copyFile(pomPath, destPomPath)
	if err != nil {
		return fmt.Errorf("failed to copy POM file: %w", err)
	}

	// Copy JAR file
	destJarPath := filepath.Join(artifactDir, jarFileName)
	if fileExists(destJarPath) && !preRelease {
		return fmt.Errorf("JAR file already exists at %s and version is not a pre-release", destJarPath)
	}
	err = copyFile(jarPath, destJarPath)
	if err != nil {
		return fmt.Errorf("failed to copy JAR file: %w", err)
	}
	fmt.Printf("Successfully published %s:%s:%s to local repository\n", groupID, artifactID, version)
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	// What are the other errors that can happen ... bad path? No permission?  How likely are those
	// to occur given the repo is in the user's home dir?  Not likely enough for me to want to add
	// 3 lines of error checking to every call to see if a path exists, but I also don't want to
	// quietly ignore an error and be stumped why something is not working.
	panic(err)
}

// copyFile is a helper function to copy a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}
	return nil
}

func fetchFromRemote(mavenBaseURL string, groupID, artifactID, version, file string, out io.Writer) error {
	groupIDWithSlashes := strings.ReplaceAll(groupID, ".", "/")
	u, err := url.Parse(mavenBaseURL)
	if err != nil {
		return fmt.Errorf("failed to parse maven Coordinates: %w", err)
	}
	u.Path = path.Join(u.Path, groupIDWithSlashes, artifactID, version, file)
	fileURL := u.String()
	fmt.Printf("Fetching %s\n", fileURL)
	resp, err := http.Get(fileURL)
	if err != nil {
		return fmt.Errorf("error downloading %s: %v", fileURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download %s: %s", fileURL, resp.Status)
	}
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("error saving %s: %v", fileURL, err)
	}
	//fmt.Printf("Successfully downloaded %s\n", fileURL)
	return nil
}
