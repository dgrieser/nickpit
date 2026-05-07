# TODO

- Cleanup: Reasoning flickering, Reasoning block should be 10 lines even if less lines of content.
- Logging: Change to "[agent: <agent_name>]"
- Logging: Put round into bracket, e.g. "[agent: <agent_name>, round: <round>]"
- Logging: Fix wrong Reasoning Done progress message.
- Verbose: Fix json formatted printing for verbose on <wrongcolor>"content": "</wrongcolor>

- Add reasoning loop detection and abort if loop is detected, temporarily retry with lower reasoning effort
- Add max turns for agents
- Add suggestion agent to format suggestions for gitlab/github
- Add simplifier agent to shorten text on findings
- Add kubernetes best practices styleguide: https://github.com/wshobson/agents/tree/main/plugins/kubernetes-operations/skills
- Add more styleguides: https://github.com/wshobson/agents/tree/main/plugins
