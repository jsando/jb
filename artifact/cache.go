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
	Packaging    string       `xml:"packaging"`
	Dependencies []Dependency `xml:"dependencies>dependency"`
}

type Dependency struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
	Scope      string `xml:"scope"`
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
