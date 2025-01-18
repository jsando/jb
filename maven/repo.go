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
	"strings"
)

const MAVEN_CENTRAL_URL = "https://repo.maven.apache.org/maven2/"

type LocalRepository struct {
	baseDir string
	remotes []string
	poms    map[string]POM
}

func OpenLocalRepository() *LocalRepository {
	return &LocalRepository{
		baseDir: "~/.jb/repository",
		remotes: []string{MAVEN_CENTRAL_URL},
		poms:    make(map[string]POM),
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
	relPath := filepath.Join(filepath.SplitList(groupIDWithSlashes)...)
	relPath = filepath.Join(relPath, artifactID, version)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(strings.ReplaceAll(jc.baseDir, "~", homeDir), relPath)
}

func (c *LocalRepository) GetPOM(groupID, artifactID, version string) (POM, error) {
	gav := GAV(groupID, artifactID, version)
	pom, found := c.poms[gav]
	if found {
		return pom, nil
	}
	pom = POM{}
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
	return pom, err
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
	fmt.Printf("mkdir -p %s\n", filepath.Dir(artifactPath))
	if err != nil {
		return "", err
	}
	outFile, err := os.OpenFile(artifactPath, os.O_CREATE|os.O_WRONLY, 0600)
	defer outFile.Close()
	if err != nil {
		return artifactPath, err
	}
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
	pomFileName := filepath.Base(pomPath)
	jarFileName := filepath.Base(jarPath)
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
	u.Path = path.Join(u.Path, filepath.Join(filepath.SplitList(groupIDWithSlashes)...), artifactID, version, file)
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
	fmt.Printf("Successfully downloaded %s\n", fileURL)
	return nil
}
