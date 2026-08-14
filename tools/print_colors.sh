#!/usr/bin/env bash
#
# print_colors.sh — reference sheet for every colour the NickPit terminal UI uses.
#
# It reproduces the palette and the rendering rules of:
#   internal/logging/progress.go  — the --show-progress line palette
#   internal/logging/live.go      — the live dashboard (header, bars, footer)
#   internal/logging/reasoning_renderer.go — the streamed reasoning block
#   internal/output/terminal.go, internal/output/badge.go — review output
#
# Each entry shows the SGR code, the Go constant it comes from, and where the
# colour actually appears. Requires a truecolor-capable terminal: the stage
# column, progress bars, badges and the animated wordmark are all 24-bit; the
# message/bracket palette is 256-colour.
#
# Usage: tools/print_colors.sh

set -uo pipefail

ESC=$'\033'
RESET="${ESC}[0m"

# s CODE TEXT — wrap TEXT in an SGR sequence, like logging.progressStyle.
s() { printf '%s[%sm%s%s' "$ESC" "$1" "$2" "$RESET"; }

grey() { s '38;5;244' "$1"; }    # progressGrey
light() { s '38;5;252' "$1"; }   # progressLight

rule() { local n=$1 out=''; local i; for (( i = 0; i < n; i++ )); do out+='─'; done; printf '%s' "$out"; }

heading() {
  printf '\n%s\n%s\n' "$(s '1;38;5;255' "$1")" "$(s '38;5;242' "$(rule 76)")"
}

# swatch CODE CONSTANT USAGE — one palette row.
swatch() {
  printf '  %s  %s  %s  %s\n' \
    "$(s "$1" '████')" \
    "$(printf '%-12s' "$1")" \
    "$(printf '%-30s' "$2")" \
    "$(grey "$3")"
}

################################################################################
heading 'Stage column — progressStageStyles (bold, first 10 columns of every progress line)'
################################################################################

stage_row() { # NAME CODE TONE USAGE
  printf '  %s  %s  %s  %s\n' \
    "$(s "$2" "$(printf '%-10s' "$1")")" \
    "$(printf '%-19s' "$2")" \
    "$(printf '%-16s' "$3")" \
    "$(grey "$4")"
}

# Truecolor, not the 256-colour cube: 17 stages need more room than xterm-256
# offers before names start looking alike. Every entry keeps L* ≥ 64 (readable on
# a dark background) and no two are closer than ΔE*ab ≈ 25 in CIELAB — the old
# 256-colour set had pairs as close as ΔE 8 (Model/Chat).
stage_row NickPit    '1;38;2;252;104;176' 'pink-magenta'  'startup banner, version, config resolution'
stage_row Model      '1;38;2;140;206;252' 'pale sky blue'   'model/endpoint resolution, alias mapping'
stage_row Agent      '1;38;2;88;156;252'  'azure blue'    'agent + workflow identity; also the live lane name'
stage_row ModelCheck '1;38;2;20;252;144'  'spring green'  'reachability / capability probe of an endpoint'
stage_row Review     '1;38;2;252;152;172' 'rose'          'review context: mode, profile, threshold, target'
stage_row Chat       '1;38;2;172;140;236' 'lavender'      'chat mode turns'
stage_row Request    '1;38;2;252;168;24'  'amber'         'request sent to the LLM (state=sent)'
stage_row Reasoning  '1;38;2;252;88;220'  'magenta'       'reasoning stream lifecycle'
stage_row Response   '1;38;2;100;184;20'  'leaf green'    'response received (state=done)'
stage_row Tool       '1;38;2;20;244;216'  'turquoise'     'tool call → result lines'
stage_row Result     '1;38;2;228;228;20'  'yellow'        'per-agent outcome'
stage_row Publish    '1;38;2;252;112;76'  'coral'         'publishing to GitHub/GitLab'
stage_row Categorize '1;38;2;244;184;252' 'pale orchid'   'finding categorisation'
stage_row Verify     '1;38;2;0;172;176'   'deep teal'       'verification / refutation pass'
stage_row Finalize   '1;38;2;100;172;108' 'sage green'    'dedupe, merge, filter'
stage_row Verdict    '1;38;2;208;120;252' 'violet'        'overall correctness verdict'
stage_row Summarize  '1;38;2;240;216;140' 'pale gold'     'summary generation'
printf '  %s\n' "$(grey 'unknown stage → 1;38;5;252 (bold light grey) fallback')"

################################################################################
heading 'State word — stateColor()'
################################################################################

swatch '38;5;252' 'progressColorLightGrey'  'done, ok, ready (and the default)'
swatch '38;5;203' 'progressColorErrorRed'   'error'
swatch '38;5;221' 'progressColorWarnYellow' 'retry, warn'
swatch '38;5;177' 'progressColorSkipPurple' 'skip'
printf '  %s\n' "$(grey 'start / sent carry no own colour — they fall through to light grey')"

################################################################################
heading 'Message + bracket palette — progress.go colour constants'
################################################################################

swatch '38;5;244' 'progressColorGrey'              'brackets, " · ", ":", "/", "@", "→", version'
swatch '38;5;242' 'progressColorDarkGrey'          'reserved dim tone (rules, chrome)'
swatch '38;5;252' 'progressColorLightGrey'         'plain words, detail text, units'
swatch '38;5;255' 'progressColorWhite'             'bold status word in the final header, "Findings" label'
swatch '38;5;118' 'progressColorNumberGreen'       'every number: counts, turns, percentages, N/M'
swatch '38;5;71'  'progressColorUnitGreen'         'number affixes: "#", "/", "≤", "≥", "∞", trailing unit'
swatch '38;5;71'  'progressColorHashDarkGreen'     'git SHAs (≥7 hex) and UUIDs, coloured as one token'
swatch '38;5;116' 'progressColorKeyTurquoise'      'key= in key=value, "@alias" model prefix, elapsed time'
swatch '38;5;37'  'progressColorKeyTeal'           'agent role, model name, workflow name'
swatch '38;5;120' 'progressColorStringGreen'       '"quoted strings" (escape-aware)'
swatch '38;5;156' 'progressColorBoolGreen'         'true / false'
swatch '38;5;218' 'progressColorTaskPink'          'agent name, reasoning effort, repo name'
swatch '38;5;110' 'progressColorMutedModel'        'model+effort when a role already owns the bracket'
swatch '38;5;105' 'progressColorURLPurpleBlue'     'endpoint base URL after " @ "'
swatch '38;5;216' 'progressColorProfile'           'review profile name; "filtered" in the live footer'
swatch '38;5;214' 'progressColorBranchFromGold'    'head branch in "repo @ head → base"'
swatch '38;5;48'  'progressColorBranchToAquaGreen' 'base branch in "repo @ head → base"'
swatch '38;5;203' 'progressColorErrorRed'          'error state, failure mark, "dropped", "refuted"'
swatch '38;5;221' 'progressColorWarnYellow'        'warn / retry state'
swatch '38;5;177' 'progressColorSkipPurple'        'skip state'
swatch '38;5;179' 'dupGold (live.go, local)'       '"duplicate" label in the live footer'
swatch '3;90'     'reasoning_renderer.go'          'italic dim grey — the streamed reasoning block'
swatch '2'        'output/terminal.go Dim'         'dim review-output chrome + the closing rule'
swatch '33'       'output/terminal.go Warn'        'legacy yellow warnings in review output'

################################################################################
heading 'Example --show-progress lines'
################################################################################

turn() { # N  → formatProgressTurn
  printf '%s%s' "$(s '38;5;71' '#')" "$(s '38;5;118' "$1")"
}

printf '  %s %s %s %s %s\n' \
  "$(s '1;38;2;252;104;176' "$(printf '%-10s' NickPit)")" \
  "$(grey '[')$(s '38;5;216' 'go')$(grey ', ')$(s '38;5;71' '≤')$(s '38;5;118' '2')$(grey ']')" \
  "$(light start)" \
  "$(s '38;5;116' 'version')$(grey '=')$(light 'v0.0.14')" \
  "$(s '38;5;71' 'a1b2c3d4e5f6')"

printf '  %s %s %s %s\n' \
  "$(s '1;38;2;140;206;252' "$(printf '%-10s' Model)")" \
  "$(grey '[')$(s '38;5;116' '@big') $(s '38;5;37' 'Qwen3.6-480B')$(grey ':')$(s '38;5;218' 'high')$(grey ' @ ')$(s '38;5;105' 'https://llm.internal')$(grey ']')" \
  "$(light ready)" \
  "$(s '38;5;118' '262144')$(s '38;5;71' 'tok')$(grey ' · ')$(s '38;5;156' 'true')"

printf '  %s %s %s %s\n' \
  "$(s '1;38;2;88;156;252' "$(printf '%-10s' Agent)")" \
  "$(grey '[')$(s '38;5;37' 'review')$(grey ': ')$(s '38;5;218' 'security')$(grey '/')$(s '38;5;218' 'auth')$(grey ' · ')$(s '38;5;110' 'Qwen3.6-480B')$(grey ':')$(s '38;5;110' 'high')$(grey ' · ')$(s '38;5;37' 'full-review')$(grey ' · ')$(light 'workflow.yaml')$(grey ' · ')$(s '38;5;118' '4') $(light steps)$(grey ']')" \
  "$(turn 2)" \
  "$(light start)"

printf '  %s %s %s\n' \
  "$(s '1;38;2;252;152;172' "$(printf '%-10s' Review)")" \
  "$(light diff)$(grey ':')$(light staged)" \
  "$(grey '[')$(s '38;5;216' 'go')$(grey ', ')$(s '38;5;71' '≤')$(s '38;5;118' '2')$(grey ']') $(light on) $(s '38;5;218' 'nickpit')$(grey ' @ ')$(s '38;5;214' 'ui')$(grey '/')$(s '38;5;214' 'colors')$(grey ' → ')$(s '38;5;48' 'main')"

printf '  %s %s %s %s\n' \
  "$(s '1;38;2;20;244;216' "$(printf '%-10s' Tool)")" \
  "$(grey '[')$(s '38;5;37' 'review')$(grey ': ')$(s '38;5;218' 'security')$(grey ']')" \
  "$(turn 3)" \
  "$(s '38;5;116' 'read_file')$(grey '(')$(s '38;5;120' '"internal/logging/live.go"')$(grey ') → ')$(s '38;5;118' '1254') $(s '38;5;71' 'lines')"

printf '  %s %s %s %s\n' \
  "$(s '1;38;2;100;172;108' "$(printf '%-10s' Finalize)")" \
  "$(grey '[')$(s '38;5;37' 'dedupe')$(grey ']')" \
  "$(light 'done')" \
  "$(s '38;5;118' '7') $(light 'kept')$(grey ', ')$(s '38;5;118' '2') $(s '38;5;203' 'dropped')$(grey ', ')$(s '38;5;116' 'ratio')$(grey '=')$(s '38;5;118' '0.78')"

printf '  %s %s %s\n' \
  "$(s '1;38;2;252;168;24' "$(printf '%-10s' Request)")" \
  "$(turn 4)" \
  "$(s '38;5;221' retry) $(light 'rate limited, backoff') $(s '38;5;118' '30')$(s '38;5;71' 's')"

printf '  %s %s %s\n' \
  "$(s '1;38;2;0;172;176' "$(printf '%-10s' Verify)")" \
  "$(turn 1)" \
  "$(s '38;5;177' skip) $(light 'below threshold')"

printf '  %s %s %s\n' \
  "$(s '1;38;2;100;184;20' "$(printf '%-10s' Response)")" \
  "$(turn 5)" \
  "$(s '38;5;203' error) $(light 'context deadline exceeded after') $(s '38;5;118' '600')$(s '38;5;71' 's')$(grey ' · ')$(s '38;5;71' '∞')"

################################################################################
heading 'Live dashboard chrome'
################################################################################

# Animated "NickPit" wordmark: five cyclic truecolor gradients, one stage each,
# cross-fading into the next. Stage colours are computed like nickPitStageColor();
# this prints a static frame-0 snapshot of every stage.

GRADIENTS=(
  '255,66,106 255,128,170 255,176,120'   # rose → pink → coral
  '56,128,255 80,210,255 120,255,224'    # blue → cyan → aqua
  '72,210,130 192,240,120 110,224,200'   # green → lime → teal
  '255,150,66 255,214,96 255,118,176'    # orange → gold → magenta
  '150,96,255 198,120,255 255,122,214'   # indigo → violet → magenta
  '40,200,190 120,235,170 210,245,120'   # teal → mint → chartreuse
  '255,80,90 255,150,60 255,215,100'     # crimson → amber → gold
  '255,100,200 170,100,255 80,150,255'   # magenta → purple → azure
  '120,255,215 80,185,255 190,150,255'   # aqua → sky → lilac
  '255,205,90 175,230,90 70,210,175'     # gold → lime → jade
)
GRADIENT_NAMES=(
  'rose → pink → coral'
  'blue → cyan → aqua'
  'green → lime → teal'
  'orange → gold → magenta'
  'indigo → violet → magenta'
  'teal → mint → chartreuse'
  'crimson → amber → gold'
  'magenta → purple → azure'
  'aqua → sky → lilac'
  'gold → lime → jade'
)

# smoothstep on per-mille input, per-mille output: t²(3-2t)
smoothstep_pm() { printf '%d' $(( $1 * $1 * (3000 - 2 * $1) / 1000000 )); }

# lerp_rgb "r,g,b" "r,g,b" t_pm → "r;g;b"
lerp_rgb() {
  local ar ag ab br bg bb t=$3
  IFS=, read -r ar ag ab <<<"$1"
  IFS=, read -r br bg bb <<<"$2"
  printf '%d;%d;%d' \
    $(( ar + (br - ar) * t / 1000 )) \
    $(( ag + (bg - ag) * t / 1000 )) \
    $(( ab + (bb - ab) * t / 1000 ))
}

# sample_gradient GRADIENT t_pm → "r;g;b" (cyclic, smoothstepped between stops)
sample_gradient() {
  local stops
  read -r -a stops <<<"$1"
  local n=${#stops[@]} t=$2 pos idx frac
  pos=$(( t * n ))
  idx=$(( pos / 1000 ))
  frac=$(( pos % 1000 ))
  lerp_rgb "${stops[$(( idx % n ))]}" "${stops[$(( (idx + 1) % n ))]}" "$(smoothstep_pm "$frac")"
}

WORD='NickPit'
wordmark() { # GRADIENT_INDEX
  local out='' j rgb
  for (( j = 0; j < ${#WORD}; j++ )); do
    rgb=$(sample_gradient "${GRADIENTS[$1]}" $(( j * 1000 / ${#WORD} )))
    out+="$(s "1;38;2;${rgb}" "${WORD:j:1}")"
  done
  printf '%s' "$out"
}

for i in "${!GRADIENTS[@]}"; do
  printf '  %s  %s  %s\n' "$(wordmark "$i")" \
    "$(printf '%-9s' "stage $i")" \
    "$(grey "38;2 truecolor gradient — ${GRADIENT_NAMES[$i]}")"
done
printf '  %s\n' "$(grey 'odd stages glide a highlight over the ramp instead (grad[0]×0.4 → last stop); the last 18 of every 48 frames cross-fade into the next stage')"

printf '\n'
printf '  %s %s%s%s%s\n' "$(wordmark 1)" \
  "$(grey 'v0.0.14')" \
  "$(grey ' · ')$(s '38;5;116' '01:42')" \
  "$(grey ' · ')$(s '38;5;37' 'Qwen3.6-480B')$(grey ':')$(s '38;5;218' 'high')" \
  "$(grey ', ')$(s '38;5;116' '@small') $(s '38;5;37' 'Qwen3.6-35B')"
printf '  %s\n' "$(grey 'header — wordmark = motion cue, version dim grey, elapsed turquoise, models teal with pink effort')"

printf '\n'
printf '  %s%s%s\n' \
  "$(s '38;5;218' 'nickpit')$(grey ' @ ')" \
  "$(s '38;5;214' 'ui')$(grey '/')$(s '38;5;214' 'colors')$(grey ' → ')" \
  "$(s '38;5;48' 'main')"
printf '  %s\n' "$(grey 'target line — repo pink, head gold, base aqua-green (shared with --show-progress)')"

printf '\n'
printf '  %s %s%s\n' \
  "$(s '1;38;2;88;156;252' 'security-lane')" \
  "$(s '38;5;118' '3')$(grey '/')$(s '38;5;118' '5')" \
  "$(grey ' · ')$(s '38;5;37' 'full-review')$(grey ' · ')$(light 'workflow.yaml')$(grey ' · ')$(s '38;5;118' '4') $(light steps)"
printf '  %s  %s\n' \
  "$(s '38;5;118' '3') $(light lanes)$(grey ' · ')$(s '38;5;37' 'full-review')" \
  "$(grey 'several unnamed lanes → green count + light "lanes"')"
printf '  %s\n' "$(grey 'lane/step line — name uses the bold Agent stage colour (1;38;2;88;156;252); unit N/M green with a grey slash')"

printf '\n'
printf '  %s%s%s%s%s\n' \
  "$(s '38;5;255' 'Findings') $(s '38;5;118' '9')$(grey ' · ')" \
  "$(s '38;5;203' 'refuted') $(s '38;5;118' '2')$(grey ' · ')" \
  "$(s '38;5;179' 'duplicate') $(s '38;5;118' '1')$(grey ' · ')" \
  "$(s '38;5;216' 'filtered') $(s '38;5;118' '1')$(grey ' · ')" \
  "$(s '38;5;118' 'final') $(s '38;5;118' '5')"
printf '  %s\n' "$(grey 'findings footer — label colours are semantic, every count is green, dots grey')"

printf '\n'
printf '  %s %s %s%s%s\n' \
  "$(s '1;38;5;118' '✓')" \
  "$(s '1;38;5;255' 'Review complete')" \
  "$(grey 'v0.0.14')$(grey ' · ')" \
  "$(s '38;5;116' '02:31')$(grey ' · ')" \
  "$(s '38;5;118' '5') $(light findings)"
printf '  %s %s %s%s\n' \
  "$(s '1;38;5;203' 'x')" \
  "$(s '1;38;5;255' 'Review stopped')" \
  "$(grey 'v0.0.14')$(grey ' · ')" \
  "$(s '38;5;116' '00:12')"
printf '  %s\n' "$(grey 'frozen snapshot header — bold green ✓ / bold red "x" (same glyphs as the verdict badges), bold white status word, dim version, turquoise elapsed')"

printf '\n  %s\n' "$(s '2' "$(rule 60)")"
printf '  %s\n' "$(grey 'closing rule under the frozen dashboard — SGR 2 (dim), full terminal width')"

################################################################################
heading 'Agent progress bars — liveAgentPastelRGB (10 colours, assigned round-robin by start order)'
################################################################################

# Each colour is used four ways inside one bar:
#   fill background = the colour itself           (48;2)
#   fill text       = the colour × 0.22 — dark    (38;2)
#   remainder text  = the colour itself — light   (38;2)
#   remainder bg    = the colour × 0.42 — dark    (48;2)
PASTEL=(
  '177,185,249:periwinkle'
  '166,209,137:green'
  '244,184,228:pink'
  '129,200,190:teal'
  '198,160,246:lavender'
  '238,212,159:yellow'
  '138,173,244:blue'
  '250,157,169:rose'
  '121,247,199:mint'
  '100,208,250:sky'
)

BAR_WIDTH=44   # liveProgressBarWidth

scale_rgb() { # "r,g,b" percent → "r;g;b"
  local r g b
  IFS=, read -r r g b <<<"$1"
  printf '%d;%d;%d' $(( (r * $2 + 50) / 100 )) $(( (g * $2 + 50) / 100 )) $(( (b * $2 + 50) / 100 ))
}

rgb_sgr() { local r g b; IFS=, read -r r g b <<<"$1"; printf '%d;%d;%d' "$r" "$g" "$b"; }

# progress_bar LABEL PERCENT COLOR_INDEX — mirrors logging.progressBar().
progress_bar() {
  local label=$1 percent=$2 idx=$3
  local entry=${PASTEL[$(( idx % ${#PASTEL[@]} ))]}
  local colour=${entry%%:*}
  local right label_width n text total filled percent_start
  right=$(printf ' %3d%%' "$percent")           # 5 columns: "   0%" … " 100%"
  label_width=$(( BAR_WIDTH - 2 - ${#right} ))  # 37 — one space of inset per end
  n=${#label}
  if (( n > label_width )); then
    label="${label:0:label_width-1}…"           # padOrTrim ellipsises
  else
    label="${label}$(printf '%*s' $(( label_width - n )) '')"
  fi
  text=" ${label}${right} "
  total=${#text}
  filled=$(( (percent * total + 50) / 100 ))
  (( filled < 0 )) && filled=0
  (( filled > total )) && filled=$total
  # The percentage suffix never straddles two backgrounds: a partial fill stops
  # at the label's end, only a complete bar tints the digits too.
  percent_start=$(( 1 + label_width ))
  (( filled > percent_start && filled < total )) && filled=$percent_start

  local fill_fg fill_bg base_fg base_bg
  fill_fg="38;2;$(scale_rgb "$colour" 22)"
  fill_bg="48;2;$(rgb_sgr "$colour")"
  base_fg="38;2;$(rgb_sgr "$colour")"
  base_bg="48;2;$(scale_rgb "$colour" 42)"

  local out='' last='' i ch fg bg weight sgrv
  for (( i = 0; i < total; i++ )); do
    ch=${text:i:1}
    if (( i < filled )); then fg=$fill_fg; bg=$fill_bg; else fg=$base_fg; bg=$base_bg; fi
    weight=''
    # Only the percentage digits are bold — label, spaces and "%" stay regular.
    if (( i >= percent_start )) && [[ $ch == [0-9] ]]; then weight='1;'; fi
    sgrv="${weight}${fg};${bg}"
    if [[ $sgrv != "$last" ]]; then out+="${ESC}[0m${ESC}[${sgrv}m"; last=$sgrv; fi
    out+=$ch
  done
  printf '%s%s' "$out" "$RESET"
}

# phase_cell TURN NUDGE_INDEX NUDGE_TOTAL — the 18-column turn/nudge column.
phase_cell() {
  local t=$1 ni=$2 nt=$3 visible styled pad
  visible=$(printf '#%-2d' "$t")
  (( nt > 0 )) && visible=$(printf '#%-2d · nudges %d/%d' "$t" "$ni" "$nt")
  styled="$(turn "$t")$(printf '%*s' $(( ${#t} >= 2 ? 0 : 2 - ${#t} )) '')"
  if (( nt > 0 )); then
    styled+="$(grey ' · ')$(light 'nudges ')$(s '38;5;118' "$ni")$(grey '/')$(s '38;5;118' "$nt")"
  fi
  pad=$(( 18 - ${#visible} ))
  (( pad < 0 )) && pad=0
  printf '%s%*s' "$styled" "$pad" ''
}

# agent_row LABEL PERCENT INDEX TURN NUDGE_I NUDGE_N ELAPSED LIMIT
agent_row() {
  printf '  %s  %s  %s\n' \
    "$(progress_bar "$1" "$2" "$3")" \
    "$(phase_cell "$4" "$5" "$6")" \
    "$(grey "$7 / $8")"
}

agent_row 'review: security'          0   0 1 0 0 '00:03' '10:00'
agent_row 'review: correctness'      12   1 2 0 0 '00:41' '10:00'
agent_row 'review: performance'      37   2 3 1 3 '01:18' '10:00'
agent_row 'review: style'            48   3 2 0 0 '02:05' '∞'
agent_row 'verify: refute-finding-3' 55   4 1 0 0 '00:22' '05:00'
agent_row 'categorize'               68   5 4 2 2 '03:47' '10:00'
agent_row 'summarize'                77   6 6 0 0 '04:12' '10:00'
agent_row 'review: api-surface'      84   7 3 0 0 '02:58' '10:00'
agent_row 'verify: refute-finding-7' 92   8 2 0 0 '00:51' '05:00'
agent_row 'publish: gitlab-mr-4821' 100   9 7 0 0 '04:30' '10:00'
printf '  %s\n' "$(grey 'one row per concurrent agent — colour index = start order, wrapping after 10; bar 44 columns wide, label inside, percentage pinned right, only the digits bold')"
printf '  %s\n' "$(grey 'right of the bar: turn "#N" (dark-green #, bright-green number), optional nudge N/M, then grey "elapsed / limit" (∞ when no deadline)')"

printf '\n'
printf '  %s\n' "$(light 'Same agent, fill sweeping across the label — the percentage stays on one background until 100%:')"
for pct in 0 8 25 50 75 92 100; do
  if (( pct == 100 )); then note='complete — fill covers the digits too'; else note='partial — fill stops before the digits'; fi
  printf '  %s  %s\n' "$(progress_bar 'review: security · auth' "$pct" 4)" \
    "$(grey "$(printf '%3d%% — %s' "$pct" "$note")")"
done

printf '\n'
printf '  %s\n' "$(light 'Long labels are ellipsised by padOrTrim, so the bar width never changes:')"
printf '  %s\n' "$(progress_bar 'review: internal/logging/live.go · concurrency-and-rendering' 44 5)"

printf '\n'
printf '  %s\n' "$(grey 'animation: while a turn runs the fill creeps 90% of the way toward the next step (15s time constant); once a step lands it catches up with a 0.5s constant. It only ever moves forward, and done pins it at 100%.')"

################################################################################
heading 'Fallback line styling — styleLiveLines() (rows carrying no colour of their own)'
################################################################################

swatch '1;38;2;140;206;252' 'row 0 (header)'    'bold pale sky — same code as the Model stage'
swatch '38;5;110'  'row 1 (live only)' 'muted model tone for the second line'
swatch '38;5;118'  'last row'          'green — the findings footer'
swatch '38;5;252'  'every other row'   'light grey'

################################################################################
heading 'Reasoning stream — reasoning_renderer.go'
################################################################################

printf '  %s\n' "$(s '3;90' 'Reasoning for review: security...')"
printf '  %s\n' "$(s '3;90' 'Checking whether the token expiry comparison is inclusive…')"
printf '  %s\n' "$(grey 'italic dim grey (3;90) for the whole streamed block, so reasoning never competes with progress lines')"

################################################################################
heading 'Review output badges — output/badge.go (truecolor background, black text)'
################################################################################

badge() { printf '%s[48;2;%sm%s[38;2;0;0;0m%s%s' "$ESC" "$1" "$ESC" "$2" "$RESET"; }

# verdict_badge COLOUR WORD GLYPH — the glyph is bold (SGR 1, then 22 to end the
# bold without dropping the badge's fore/background), and "x" is plain ASCII so
# it occupies exactly one cell in every font.
verdict_badge() {
  local colour=$1 word=$2 glyph=$3 pad
  pad=$(( 16 - ${#word} - 3 ))
  (( pad < 0 )) && pad=0
  printf '%s[48;2;%sm%s[38;2;0;0;0m%*s%s %s[1m%s%s[22m %s' \
    "$ESC" "$colour" "$ESC" "$pad" '' "$word" "$ESC" "$glyph" "$ESC" "$RESET"
}

badge_row() { printf '  %s  %s  %s\n' "$1" "$(printf '%-12s' "$2")" "$(grey "$3")"; }

badge_row "$(badge '255;7;58'   '    BLOCKING    ')"     'P0 #FF073A' 'priority rank 0'
badge_row "$(badge '251;20;139' '      HIGH      ')"     'P1 #FB148B' 'priority rank 1'
badge_row "$(badge '255;81;0'   '     MEDIUM     ')"     'P2 #FF5100' 'priority rank 2'
badge_row "$(badge '255;234;0'  '      LOW       ')"     'P3 #FFEA00' 'priority rank 3'
badge_row "$(verdict_badge '0;255;13' 'CORRECT' '✓')"    'correct'    'overall verdict: correct'
badge_row "$(verdict_badge '255;7;58' 'INCORRECT' 'x')"  'incorrect'  'overall verdict: incorrect'
printf '  %s\n' "$(grey 'fixed 16-column width, mirroring the published badge SVGs in assets/; verdict glyph bold, "x" ASCII so it never overflows its cell')"

printf '\n%s\n' "$(grey 'Without a TTY (or with useANSI=false) every element above degrades to the same text with no escape sequences.')"
