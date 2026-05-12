package proofread

import (
	"fmt"
	"strings"
)

// buildSystemPrompt is the instructions we hand the LLM. The brief is
// deliberately tight:
//   - fixes[] lists *observed errors* in the text the user already has
//   - newTerms[] surfaces *candidate glossary entries* we think should
//     live in the user's personal dictionary
//   - nothing else (no style rewriting, no summarisation, no commentary)
//
// We pass the current glossary summary so the model doesn't re-suggest
// entries that already exist, and so it knows what the user's preferred
// canonical form is for each known term.
func buildSystemPrompt(glossarySummary string) string {
	return `你是一个严格的中文语音转写稿校对助手。用户的文字稿来自 Whisper 模型的自动转写，已经过一次确定性词表替换。你的任务：

1. 只修改明确的错误，不重写风格、不扩写内容、不调整段落。
2. 常见错误类型：
   - 同音字误识别（比如"换面 → 幻灭"、"全往 → 全网"、"拖负 → 托付"、"空得住 → 控得住"）
   - 专有名词未识别（人名、产品名、技术术语、公司名）
   - 标点与格式（根据语气补句号/问号/感叹号；去掉多余空格）
3. 识别文中反复出现、疑似专有名词或技术术语、但**不在词表中**的 token，建议加入词表。
4. 严格输出 JSON，不要 markdown 代码块、不要额外说明。
5. 没有问题就输出空数组。不要硬凑。

## 当前词表摘要

` + glossarySummary + `

## 输出 schema（严格遵守）

{
  "fixes": [
    {
      "segmentIndex": <int>,            // 对应输入的 segment 下标
      "original": "<文本里的原片段>",
      "suggested": "<建议替换的内容>",
      "reason": "<一句话说明为什么>",
      "type": "homophone" | "punctuation" | "term" | "grammar" | "other"
    }
  ],
  "newTerms": [
    {
      "term": "<建议收录的正确形式>",
      "wrongs": ["<转写稿里出现的几种错写>"],
      "evidence": "<从原文中截取的证据段>",
      "confidence": 0.0..1.0
    }
  ]
}

## 输入（segments JSON）

`
}

// formatGlossarySummary collapses the current glossary into a compact
// "right: wrong1, wrong2" listing. We cap each entry's wrongs list to
// avoid blowing up the prompt when a user has hundreds of variants.
func formatGlossarySummary(g *Glossary) string {
	entries := g.List("")
	if len(entries) == 0 {
		return "(空)"
	}
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "- %s", e.Right)
		if len(e.Wrong) > 0 {
			limited := e.Wrong
			if len(limited) > 6 {
				limited = limited[:6]
			}
			fmt.Fprintf(&b, "（错写：%s）", strings.Join(limited, " / "))
		}
		if e.Category != "" {
			fmt.Fprintf(&b, " [%s]", e.Category)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
