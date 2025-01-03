package artifact

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const MAVEN_CENTRAL_URL = "https://repo.maven.apache.org/maven2/"

type ID struct {
	GroupID, ArtifactID, Version string
}

type POM struct {
	XMLName           xml.Name     `xml:"project"`
	Xmlns             string       `xml:"xmlns,attr"`              // Default namespace
	XmlnsXsi          string       `xml:"xmlns:xsi,attr"`          // XML Schema namespace
	XsiSchemaLocation string       `xml:"xsi:schemaLocation,attr"` // Schema location attribute
	ModelVersion      string       `xml:"modelVersion"`
	Packaging         string       `xml:"packaging"`
	GroupID           string       `xml:"groupId"`          // GroupID is optional if <parent> is specified
	ArtifactID        string       `xml:"artifactId"`       // ArtifactID is required
	Version           string       `xml:"version"`          // Version is required
	Parent            *Dependency  `xml:"parent,omitempty"` // Optional parent module
	Name              string       `xml:"name,omitempty"`
	Description       string       `xml:"description,omitempty"`
	URL               string       `xml:"url,omitempty"`
	Properties        *Properties  `xml:"properties,omitempty"`
	Dependencies      []Dependency `xml:"dependencies>dependency"`
}

type Dependency struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
	Scope      string `xml:"scope"`
}

type Properties struct {
	Properties []Property `xml:",any"` // Collection of key/value pairs
}

type Property struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type JarCache struct {
	baseDir string
	remotes []string
	poms    map[string]POM
}

func NewJarCache() *JarCache {
	return &JarCache{
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

func (jc *JarCache) artifactDir(groupID, artifactID, version string) string {
	groupIDWithSlashes := strings.ReplaceAll(groupID, ".", "/")
	relPath := filepath.Join(filepath.SplitList(groupIDWithSlashes)...)
	relPath = filepath.Join(relPath, artifactID, version)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(strings.ReplaceAll(jc.baseDir, "~", homeDir), relPath)
}

func (c *JarCache) GetPOM(groupID, artifactID, version string) (POM, error) {
	gav := GAV(groupID, artifactID, version)
	pom, found := c.poms[gav]
	if found {
		return pom, nil
	}
	pom = POM{}
	pomFile := pomFile(artifactID, version)
	path, err := c.GetFile(groupID, artifactID, version, pomFile)
	if err != nil {
		return pom, err
	}
	xmlFile, err := os.Open(path)
	if err != nil {
		return pom, err
	}
	defer xmlFile.Close()
	decoder := xml.NewDecoder(xmlFile)
	err = decoder.Decode(&pom)
	if err == nil {
		c.poms[gav] = pom
	}
	return pom, err
}

func (c *JarCache) GetJAR(groupID, artifactID, version string) (string, error) {
	return c.GetFile(groupID, artifactID, version, jarFile(artifactID, version))
}

func (c *JarCache) GetFile(groupID, artifactID, version, file string) (string, error) {
	path := filepath.Join(c.artifactDir(groupID, artifactID, version), file)
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			err := os.MkdirAll(filepath.Dir(path), 0755)
			fmt.Printf("mkdir -p %s\n", filepath.Dir(path))
			if err != nil {
				return "", err
			}
			outFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
			defer outFile.Close()
			if err != nil {
				return path, err
			}
			for _, remote := range c.remotes {
				err = fetchFromMaven(remote, groupID, artifactID, version, file, outFile)
				if err == nil {
					break
				}
				fmt.Printf("error fetching from maven %s: %s\n", remote, err.Error())
			}
			// not found ... delete empty file and return error
			if err != nil {
				outFile.Close()
				os.Remove(path)
				return path, fmt.Errorf("failed to fetch %s from any remote", path)
			}
		}
	}
	return path, err
}

func (c *JarCache) PublishLocal(groupID, artifactID, version, jarPath, pomPath string) error {
	pomFileName := filepath.Base(pomPath)
	jarFileName := filepath.Base(jarPath)
	artifactDir := c.artifactDir(groupID, artifactID, version)
	preRelease := strings.Contains(version, "-")

	// Create the artifact directory if it doesn't exist
	err := os.MkdirAll(artifactDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create artifact directory: %w", err)
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
	return err == nil
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

func fetchFromMaven(mavenBaseURL string, groupID, artifactID, version, file string, out io.Writer) error {
	groupIDWithSlashes := strings.ReplaceAll(groupID, ".", "/")
	u, err := url.Parse(mavenBaseURL)
	if err != nil {
		return fmt.Errorf("failed to parse maven URL: %w", err)
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
