package maven

import "encoding/xml"

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
