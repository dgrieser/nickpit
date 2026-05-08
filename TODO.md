# TODO

- patch is incorrect on any review agent with confidence > 0.75 should programmatically add "patch is incorrect"
- Cleanup: Reasoning flickering, Reasoning block should be 10 lines even if less lines of content
- Logging: Change to "[agent: <agent_name>]"
- Logging: Put turns into bracket, e.g. "[agent: <agent_name>, turn: <round>]"
- Logging: Fix wrong Reasoning Done progress message.
- Verbose: Fix json formatted printing for verbose on <wrongcolor>"content": "</wrongcolor>

- Add max turns for agents
- Add reasoning summary agent to use for progress printing (e.g. "Reasoned about A, B and C", short)
- Add suggestion agent to format suggestions for gitlab/github
- Add simplifier agent to shorten text on findings
- Add kubernetes best practices styleguide: https://github.com/wshobson/agents/tree/main/plugins/kubernetes-operations/skills
- Add more styleguides: https://github.com/wshobson/agents/tree/main/plugins
