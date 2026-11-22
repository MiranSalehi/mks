package mks

import (
	"strings"
	"testing"
)

func TestSanitizeHTMLKeepsEditorFormatting(t *testing.T) {
	keep := map[string]string{
		`<p style="text-align: center">x</p>`:                                                      `text-align: center`,
		`<span style="color: #e11d48">x</span>`:                                                    `color: #e11d48`,
		`<span style="background-color: rgb(255, 0, 0)">x</span>`:                                  `background-color: rgb(255, 0, 0)`,
		`<span style="font-size: 18px">x</span>`:                                                   `font-size: 18px`,
		`<span style="font-family: Georgia, serif">x</span>`:                                       `font-family: Georgia, serif`,
		`<p style="margin-left: 40px">x</p>`:                                                       `margin-left: 40px`,
		`<mark class="marker-yellow">x</mark>`:                                                     `class="marker-yellow"`,
		`<p class="info-box">x</p>`:                                                                `class="info-box"`,
		`<pre class="fancy-code fancy-code-dark"><code class="language-javascript">x</code></pre>`: `class="language-javascript"`,
		`<ul data-type="taskList"><li data-checked="true" data-type="taskItem">x</li></ul>`:        `data-checked="true"`,
		`<ol style="list-style-type: upper-roman"><li>x</li></ol>`:                                 `list-style-type: upper-roman`,
		`<a href="https://example.com/a.pdf" download="file">x</a>`:                                `download="file"`,
		`<video src="/files/a.mp4" controls preload="metadata"></video>`:                           `<video src="/files/a.mp4"`,
		`<audio src="/files/a.mp3" controls></audio>`:                                              `<audio src="/files/a.mp3"`,
		`<table><tbody><tr><td colspan="2">x</td></tr></tbody></table>`:                            `colspan="2"`,
	}
	for in, want := range keep {
		if out := SanitizeHTML(in); !strings.Contains(out, want) {
			t.Errorf("SanitizeHTML(%q) = %q, want %q kept", in, out, want)
		}
	}
}

func TestSanitizeHTMLDropsUnsafeEditorInput(t *testing.T) {
	cases := map[string]string{
		`<p style="background-image: url(javascript:alert(1))">x</p>`: "background-image",
		`<span style="color: expression(alert(1))">x</span>`:          "expression",
		`<p class="evil">x</p>`:                                       "evil",
		`<video src="javascript:alert(1)" onplay="x()"></video>`:      "javascript",
		`<input type="text" value="x">`:                               `type="text"`,
		`<span style="font-family: x;}body{display:none">x</span>`:    "display",
	}
	for in, bad := range cases {
		if out := SanitizeHTML(in); strings.Contains(out, bad) {
			t.Errorf("SanitizeHTML(%q) = %q, kept %q", in, out, bad)
		}
	}
}
