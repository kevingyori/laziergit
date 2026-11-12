# guide-eval

Measures the review guide's agent contract with a cold agent, the way the
direction doc asks ("prompt tweaks don't merge without that comparison").

    go build -o /tmp/laziestgit .
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
