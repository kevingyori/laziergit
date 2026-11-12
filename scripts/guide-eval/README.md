# guide-eval

Measures the review guide's agent contract with a cold agent, the way the
direction doc asks ("prompt tweaks don't merge without that comparison").

    go build -o /tmp/laziestgit .
    go run ./scripts/guide-eval/prompts guide > prompt.txt
    go run ./scripts/guide-eval/prompts feedback > feedback-prompt.txt
    scripts/guide-eval/run.sh /tmp/laziestgit out a prompt.txt feedback-prompt.txt
    scripts/guide-eval/summarize.py out -v

`run.sh` builds the fixture repo (`fixture.sh`: a small Go gateway with a
staged, unstaged and untracked change), opens the guide once so the snapshot
exists, then runs `claude -p` with only the MCP config **M** copies and the
prompt **A** copies. After the publish it sends two reviewer comments
(`sendfeedback`) and resumes the agent with the feedback prompt. It opens the
real window twice to capture before/after screenshots.

The fixture plants two things a good guide should surface with evidence: the
per-client map never evicts, and `X-Forwarded-For` is trusted by default.
`bad-publishes.txt` is an extra instruction for one agent that tries invalid
publishes on purpose and grades the error messages.

`summarize.py` reports calls, rejected publishes, errors, budget hits and the
shape of the published guide per agent.

`baseline-*.txt` are the prompts of the contract before the rewrite.

## Results (three Haiku agents each; c tries five bad publishes on purpose)

| | calls to publish (a / b / c) | unintended rejected publishes | agents that never published |
|---|---|---|---|
| Before the rewrite | 15 / 7 / 11 | 10 | 1 of 3 |
| After, first run | 5 / 5 / 10 | 1 (malformed JSON from the model) | 0 of 3 |
| After, second run | 5 / 5 / 11 | 1 (a hunk in two sections, fixed in one retry) | 0 of 3 |
