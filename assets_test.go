package mcpcenterapp

import "testing"

func TestAssetsIncludeAppAndKit(t *testing.T) {
	assets, err := Assets()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"index.html", "app.js", "styles.css", "desktopkit/tokens.css", "desktopkit/components.css"} {
		f, err := assets.Open(path)
		if err != nil {
			t.Fatalf("open %s: %v", path, err)
		}
		_ = f.Close()
	}
}
