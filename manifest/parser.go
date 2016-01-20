package manifest

import (
	"encoding/xml"
)

var _ = xml.Unmarshal

type Copyfile struct {
	Src  string `xml:"src,attr"`
	Dest string `xml:"dest,attr"`
}

type Linkfile struct {
	Src  string `xml:"src,attr"`
	Dest string `xml:"dest,attr"`
}

type Project struct {
	Path     string     `xml:"path,attr"`
	Name     string     `xml:"name,attr"`
	Remote   string     `xml:"remote,attr"`
	Copyfile []Copyfile `xml:"copyfile"`
	Linkfile []Linkfile `xml:"linkfile"`
	Groups   string     `xml:"groups,groups"`

	Revision   string `xml:"revision,attr"`
	DestBranch string `xml:"dest-branch,attr"`
	SyncJ      string `xml:"sync-j,attr"`
	SyncC      string `xml:"sync-c,attr"`
	SyncS      string `xml:"sync-s,attr"`

	Upstream   string `xml:"upstream,attr"`
	CloneDepth string `xml:"clone-depth,attr"`
	ForcePath  string `xml:"force-path,attr"`
}

type Remote struct {
	Alias    string `xml:"alias,attr"`
	Name     string `xml:"name,attr"`
	Fetch    string `xml:"fetch,attr"`
	Review   string `xml:"review,attr"`
	Revision string `xml:"revision,attr"`
}

type Default struct {
	Revision   string `xml:"revision,attr"`
	Remote     string `xml:"remote,attr"`
	DestBranch string `xml:"dest-branch,attr"`
	SyncJ      string `xml:"sync-j,attr"`
	SyncC      string `xml:"sync-c,attr"`
	SyncS      string `xml:"sync-s,attr"`
}

type ManifestServer struct {
	URL string `xml:"url,attr"`
}
type Manifest struct {
	Default Default   `xml:"default"`
	Remote  Remote    `xml:"remote"`
	Project []Project `xml:"project"`
}
