# TODO

- Extract a tool template to reuse in all prompts
- Instruct review agents about the previous context agent run
- Cleanup: Reasoning flickering, Reasoning block should be 10 lines even if less lines of content
- Logging: Change to "[agent: <agent_name>]"
- Logging: Put turns into bracket, e.g. "[agent: <agent_name>, turn: <round>]"
- Logging: Fix wrong Reasoning Done progress message.
- Verbose: Fix json formatted printing for verbose on <wrongcolor>"content": "</wrongcolor>

- Extract language detection, file mappings and context mappings into separate yaml files in ./mappings folder and embed at build (easier to edit)
- Add reasoning loop detection, e.g. exact same line, repeated N amounts. Abort request if loop is detected, temporarily retry with lower reasoning effort until OK. Next request continues with configured reasoning effort
- Add max turns for agents
- Add reasoning summary agent to use for progress printing (e.g. "Reasoned about A, B and C", short)
- Add suggestion agent to format suggestions for gitlab/github
- Add simplifier agent to shorten text on findings
- Add kubernetes best practices styleguide: https://github.com/wshobson/agents/tree/main/plugins/kubernetes-operations/skills
- Add more styleguides: https://github.com/wshobson/agents/tree/main/plugins
