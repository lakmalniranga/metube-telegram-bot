package main

import (
	"reflect"
	"testing"
)

func TestExtractLinks(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"check this https://youtu.be/dQw4w9WgXcQ?si=abc nice", []string{"https://www.youtube.com/watch?v=dQw4w9WgXcQ"}},
		{"https://www.youtube.com/watch?v=abc123&list=PLxyz&index=3", []string{"https://www.youtube.com/watch?v=abc123"}},
		{"https://music.youtube.com/watch?v=abc123&feature=share.", []string{"https://www.youtube.com/watch?v=abc123"}},
		{"https://youtube.com/shorts/XYZ789?feature=share", []string{"https://www.youtube.com/watch?v=XYZ789"}},
		{"https://www.youtube.com/playlist?list=PL123", []string{"https://www.youtube.com/playlist?list=PL123"}},
		{"(https://youtu.be/a1) and https://youtu.be/a1 again, https://youtu.be/b2", []string{"https://www.youtube.com/watch?v=a1", "https://www.youtube.com/watch?v=b2"}},
		{"https://example.com/watch?v=abc https://notyoutube.com/ https://youtu.be/", nil},
		{"https://evil.com/?u=https://youtu.be/abc no", nil},
		{"no links here", nil},
	}
	for _, c := range cases {
		if got := extractLinks(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("extractLinks(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
