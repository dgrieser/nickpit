# TODO

- FindingMarker(id) = <!-- nickpit:finding:<uuid> --> (reviewmd/render.go:30); existingMarkers (gitlab/publish.go:94, github too) skips already-posted findings on re-run.
  But the marker key is the finding UUID, minted randomly per run. Run v3 can never match v2's markers → every re-run reposts everything as new comments.
  Only protects re-publish of the same run.

- Add feature to give additional instructions to agents
- Add max turns for agents; unlimited by default
- Add suggestion agent to format suggestions for gitlab/github
- Remove unnecessary arguments/config.

- Add man pages for commands used/called
