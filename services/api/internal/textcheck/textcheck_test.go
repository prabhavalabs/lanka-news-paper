package textcheck

import "testing"

func TestForeign(t *testing.T) {
	clean := []string{
		"අග්‍රාමාත්‍ය හරිනි අමරසූරිය ඔක්තෝබර් 11 දා අරලියගහ මන්දිරයේ; ITN News වාර්තා කරයි (2026).",
		"ශ්‍රී ලංකා · ක්‍රීඩා — “උද්ධෘත” 1,989 දෙනා",
		"இலங்கை அரசாங்கம் இன்று அறிவித்தது; BBC Tamil",
		"Parliament met today in Colombo.",
		"",
	}
	for _, text := range clean {
		if r, found := Foreign(text); found {
			t.Errorf("%q flagged at %q", text, r)
		}
	}
	dirty := []string{
		"අනාගතය වැඩිහිටියන්ගේ වගකීමක් බවද ඇය хэлсниййг; ITN News-ийн සාරාංශය",
		"2026年7月13日，斯里兰卡内阁会议作出多项决定。",
		"ශ්‍රී ලංකා ᠮᠣᠩᠭᠣᠯ පුවත",
		"සිංහල පාඨය இலங்கை සමඟ",
	}
	for _, text := range dirty {
		if _, found := Foreign(text); !found {
			t.Errorf("%q not flagged", text)
		}
	}
}
