#!/usr/bin/env python3
"""Summarizes guide-eval transcripts: tool calls, rejected publishes, errors.
Usage: summarize.py <out-dir> [name ...]   (-v prints every call)"""
import json, sys, glob, os

def calls(path):
    out, by_id = [], {}
    if not os.path.exists(path):
        return out, ""
    final = ""
    for line in open(path):
        try:
            m = json.loads(line)
        except ValueError:
            continue
        if m.get("type") == "result":
            final = m.get("result") or ""
        content = (m.get("message") or {}).get("content")
        if not isinstance(content, list):
            continue
        for c in content:
            if c.get("type") == "tool_use":
                call = {"tool": c["name"].replace("mcp__laziestgit__", ""), "input": c.get("input"), "error": None, "result": ""}
                by_id[c["id"]] = call
                out.append(call)
            elif c.get("type") == "tool_result" and c.get("tool_use_id") in by_id:
                text = c.get("content")
                if isinstance(text, list):
                    text = " ".join(x.get("text", "") for x in text)
                by_id[c["tool_use_id"]]["result"] = text or ""
                if c.get("is_error"):
                    by_id[c["tool_use_id"]]["error"] = text
    return out, final

verbose = "-v" in sys.argv
args = [a for a in sys.argv[1:] if a != "-v"]
out = args[0]
names = args[1:] or sorted({os.path.basename(p).rsplit("-publish", 1)[0] for p in glob.glob(out + "/*-publish.jsonl")})
for name in names:
    pub, final = calls(f"{out}/{name}-publish.jsonl")
    fb, fbfinal = calls(f"{out}/{name}-feedback.jsonl")
    publishes = [c for c in pub if c["tool"] == "publish_review_guide"]
    rejected = [c for c in publishes if c["error"]]
    budget = [c for c in pub + fb if c["error"] and "budget" in c["error"]]
    try:
        guide = json.load(open(f"{out}/{name}-guide.json"))
    except ValueError:
        guide = None
    print(f"== {name}")
    print(f"  publish phase: {len(pub)} calls, {len(publishes)} publishes, {len(rejected)} rejected, {sum(1 for c in pub if c['error'])} errors, {len(budget)} budget hits")
    print(f"  published: {'yes' if guide else 'NO'}")
    if guide:
        notes = [n for s in guide["sections"] for n in s.get("notes") or []]
        print(f"  guide: {len(guide['sections'])} sections, {len(notes)} notes ({sum(1 for n in notes if n['kind'] != 'explain')} assumption/question, {sum(1 for n in notes if n.get('line'))} line-anchored), {sum(1 for s in guide['sections'] if s.get('diagram'))} diagrams, provenance {guide.get('provenance')}")
    replies = [c for c in fb if c["tool"] == "reply_to_feedback"]
    print(f"  feedback phase: {len(fb)} calls, {len(replies)} replies ({', '.join(c['input'].get('kind', '?') for c in replies)}), {sum(1 for c in fb if c['error'])} errors")
    for c in pub + fb:
        if c["error"]:
            print(f"  ! {c['tool']}: {c['error'][:300]}")
    if verbose:
        for c in pub + fb:
            print(f"    {c['tool']} {json.dumps(c['input'])[:160]} -> {'ERR ' if c['error'] else ''}{c['result'][:100]!r}")
        print("  final:", final[:1500])
        print("  feedback final:", fbfinal[:1500])
