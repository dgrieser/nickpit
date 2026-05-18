# TODO

- Add max turns for agents; unlimited by default
- Add nudge N times for review agents; e.g. when review agent is done, ask it N more times to make sure it found everything; configurable; keep result before nudge; make sure IN CODE (NOT PROMPT): nudge only ADDs findings; add new findings previously reported findings; merge agent will take care of duplicates later. 
- Add suggestion agent to format suggestions for gitlab/github
- Add simplifier agent to shorten text on findings
- Add reasoning summary agent to use for progress printing (e.g. "Reasoned about A, B and C", short)


- Logging: Change to "[agent: <agent_name>]"
- Logging: Put turns into bracket, e.g. "[agent: <agent_name>, turn: <round>]"
- Logging: Fix wrong Reasoning Done progress message.
- Verbose: Fix json formatted printing for verbose on <wrongcolor>"content": "</wrongcolor>
- Remove unnecessary arguments/config.

- Add kubernetes best practices styleguide: https://github.com/wshobson/agents/tree/main/plugins/kubernetes-operations/skills
- Add more styleguides: https://github.com/wshobson/agents/tree/main/plugins
- Add man pages for commands used/called
