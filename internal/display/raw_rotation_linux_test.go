//go:build linux

package display

import "testing"

func TestExplicitRawRotationRepairsBootGeometryWhileKeepingSoftwareRotation(t *testing.T) {
	var calls [][]string
	opts := Options{FBDepthBinary: "fbdepth", BitDepth: 8, Rotation: 3, SkipRotation: true, RawRotation: true}
	err := prepareFBInkFramebuffer(opts, func(args []string, env []string) error { calls = append(calls, args); return nil })
	if err != nil || len(calls) != 2 || calls[1][1] != "-r" || calls[1][2] != "3" {
		t.Fatalf("%v %v", calls, err)
	}
}
