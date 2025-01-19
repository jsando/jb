package maven

import (
	"encoding/xml"
	"regexp"
	"strings"
)

type POM struct {
	XMLName              xml.Name              `xml:"project"`
	Xmlns                string                `xml:"xmlns,attr"`              // Default namespace
	XmlnsXsi             string                `xml:"xmlns:xsi,attr"`          // XML Schema namespace
	XsiSchemaLocation    string                `xml:"xsi:schemaLocation,attr"` // Schema location attribute
	ModelVersion         string                `xml:"modelVersion"`
	Packaging            string                `xml:"packaging"`
	GroupID              string                `xml:"groupId"`          // GroupID is optional if <parent> is specified
	ArtifactID           string                `xml:"artifactId"`       // ArtifactID is required
	Version              string                `xml:"version"`          // Version is required
	Parent               *Dependency           `xml:"parent,omitempty"` // Optional parent module
	Name                 string                `xml:"name,omitempty"`
	Description          string                `xml:"description,omitempty"`
	URL                  string                `xml:"url,omitempty"`
	Properties           *Properties           `xml:"properties,omitempty"`
	Dependencies         []Dependency          `xml:"dependencies>dependency"`
	DependencyManagement *DependencyManagement `xml:"dependencyManagement"` // parent poms can list default versions here
}

type DependencyManagement struct {
	Dependencies []Dependency `xml:"dependencies>dependency"`
}

type Dependency struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
	Type       string `xml:"type"`
	Scope      string `xml:"scope"`
}

type Properties struct {
	Properties []Property `xml:",any"` // Collection of key/value pairs
}

type Property struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

var expandVarPattern = regexp.MustCompile(`\$\{([a-zA-Z0-9._-]+)\}`)

func (p *POM) Expand(value string) string {
	result := value
	tries := 5
	for strings.Contains(result, "${") {
		// Replace function to resolve each placeholder
		result = expandVarPattern.ReplaceAllStringFunc(result, func(match string) string {
			// Extract the key within ${...}
			key := expandVarPattern.FindStringSubmatch(match)[1]
			// Look up the key in the properties map
			if value, exists := p.GetProperty(key); exists {
				return value
			}
			// Return the original placeholder if not found
			return match
		})
		tries--
		if tries == 0 {
			//fmt.Printf("failed to expand %s\n", value)
			break
		}
	}
	return result
}

func (p *POM) GetProperty(key string) (string, bool) {
	if p.Properties != nil {
		for _, prop := range p.Properties.Properties {
			if prop.XMLName.Local == key {
				return prop.Value, true
			}
		}
	}
	return "", false
}

func (p *POM) SetProperty(key, value string) {
	if p.Properties == nil {
		p.Properties = &Properties{
			Properties: make([]Property, 0),
		}
	}
	found := false
	for i, prop := range p.Properties.Properties {
		if prop.XMLName.Local == key {
			p.Properties.Properties[i].Value = value
			found = true
			break
		}
	}
	if !found {
		p.Properties.Properties = append(p.Properties.Properties, Property{
			XMLName: xml.Name{Local: key},
			Value:   value,
		})
	}
}
