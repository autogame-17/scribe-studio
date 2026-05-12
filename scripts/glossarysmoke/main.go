//go:build glossarysmoke

// Sanity-check the glossary Load + Apply pipeline on a canned string
// that mirrors the kinds of errors Whisper made on the real Agent
// video. Confirms the seed entries fire and the hit offsets line up.
//
// Usage: go run -tags glossarysmoke ./scripts/glossarysmoke/main.go
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/autogame-17/scribe-studio/backend/scribe/proofread"
)

func main() {
	g, err := proofread.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load glossary: %v\n", err)
		os.Exit(1)
	}

	// Simulated broken transcript — modeled on real whisper output.
	segs := []proofread.SegmentLike{
		{Index: 0, Text: "张浩阳介绍了evolver和evo map两个产品"},
		{Index: 1, Text: "evomap 用于构建 agent，底层依赖 open claw 和 termux"},
		{Index: 2, Text: "伊沃弗的 gep 可以让 capsule 内部的 agent 自主协作"},
		{Index: 3, Text: "vibe coding 结合 skill 是新的开发范式，大家叫它 wife coding"},
		{Index: 4, Text: "浩阳说麦奇的花园只是 gene 的一次实验"},
	}

	fmt.Println("=== before ===")
	for _, s := range segs {
		fmt.Printf("  [%d] %s\n", s.Index, s.Text)
	}

	result := g.Apply(segs)

	fmt.Println()
	fmt.Println("=== after ===")
	for _, s := range result.Segments {
		fmt.Printf("  [%d] %s\n", s.Index, s.Text)
	}

	fmt.Println()
	fmt.Printf("=== %d hits ===\n", len(result.Hits))
	raw, _ := json.MarshalIndent(result.Hits, "", "  ")
	fmt.Println(string(raw))
}
