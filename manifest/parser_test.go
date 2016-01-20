package manifest

import (
	"encoding/xml"
	"reflect"
	"testing"
)

var aospManifest = `<?xml version="1.0" encoding="UTF-8"?>
<manifest>
  <remote  name="aosp"
           fetch=".."
           review="https://android-review.googlesource.com/" />
  <default revision="master"
           remote="aosp"
           sync-j="4" />

  <project path="build" name="platform/build" groups="pdk,tradefed" >
    <copyfile src="core/root.mk" dest="Makefile" />
  </project>
  <project path="build/soong" name="platform/build/soong" groups="pdk,tradefed" >
    <linkfile src="root.bp" dest="Android.bp" />
  </project>
</manifest>`

func TestBasic(t *testing.T) {
	var manifest Manifest
	if err := xml.Unmarshal([]byte(aospManifest), &manifest); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	want := Manifest{
		Remote: Remote{
			Name:   "aosp",
			Fetch:  "..",
			Review: "https://android-review.googlesource.com/",
		},
		Default: Default{
			Revision: "master",
			Remote:   "aosp",
			SyncJ:    "4",
		},
		Project: []Project{
			{
				Path: "build",
				Name: "platform/build",
				Copyfile: []Copyfile{
					{
						Src:  "core/root.mk",
						Dest: "Makefile",
					},
				},
			},
			{
				Path: "build/soong",
				Name: "platform/build/soong",
				Linkfile: []Linkfile{
					{
						Src:  "root.bp",
						Dest: "Android.bp",
					},
				},
			},
		},
	}

	if !reflect.DeepEqual(manifest, want) {
		t.Errorf("got %v, want %v", manifest, want)
	}
}
