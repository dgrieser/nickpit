# TODO

- add a sentence to the test agent that tells it to not emit priorities above P2
- the test agent, therefore, also cannot emit patch is incorrect, add a sentence into prompt for that.
- add a feature to the JSON parser to give it certain range limitations or allowed values for a specific run, e.g. test agent JSON is invalid if it emits a finding above p2, then tell the agent that it should not emit a finding above P2


- actually "use" the verification feedback, e.g.
  - patch is incorrect on any review agent with confidence > 0.75 should programmatically add "patch is incorrect"
  - should this be another agent without tools or logic in the code?
- Logging: Change to "[agent: <agent_name>]"
- Logging: Put turns into bracket, e.g. "[agent: <agent_name>, turn: <round>]"
- Logging: Fix wrong Reasoning Done progress message.
- Verbose: Fix json formatted printing for verbose on <wrongcolor>"content": "</wrongcolor>
- Remove unnecessary arguments/config.

- Add max turns for agents
- Add reasoning summary agent to use for progress printing (e.g. "Reasoned about A, B and C", short)
- Add suggestion agent to format suggestions for gitlab/github
- Add simplifier agent to shorten text on findings
- Add kubernetes best practices styleguide: https://github.com/wshobson/agents/tree/main/plugins/kubernetes-operations/skills
- Add more styleguides: https://github.com/wshobson/agents/tree/main/plugins
