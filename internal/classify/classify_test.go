package classify

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		name  string
		files []File
		want  bool
	}{
		{"single mp4", []File{{Path: "movie.mp4", Size: 700 << 20}}, true},
		{"mkv with srt", []File{{Path: "show.mkv", Size: 1 << 30}, {Path: "show.srt", Size: 40 << 10}}, true},
		{"rar pack", []File{{Path: "rls.rar", Size: 500 << 20}, {Path: "rls.r00", Size: 500 << 20}}, false},
		{"zip only", []File{{Path: "data.zip", Size: 100 << 20}}, false},
		{"split archives", []File{{Path: "x.001", Size: 100 << 20}, {Path: "x.002", Size: 100 << 20}}, false},
		{"no video", []File{{Path: "readme.txt", Size: 1 << 10}}, false},
		{"video dominates archive", []File{{Path: "film.mkv", Size: 4 << 30}, {Path: "extra.zip", Size: 10 << 20}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(c.files)
			if got.Playable != c.want {
				t.Fatalf("Classify(%s) playable=%v, want %v (warning=%q)", c.name, got.Playable, c.want, got.Warning)
			}
			if !got.Playable && got.Warning == "" {
				t.Errorf("expected a warning for unplayable %s", c.name)
			}
		})
	}
}
