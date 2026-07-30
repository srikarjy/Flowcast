# FlowCast manual-QC-review user testing protocol

This measures the one number nothing else in the repo can honestly produce
without real people running real trials: how much faster is finding the
outlier sample in a real MultiQC report with FlowCast than without it.

No results exist yet. `scripts/time_trial.sh` records real elapsed time to
`user_testing_results.csv` (gitignored — it's collected data, not source);
this document is the protocol for producing that data. Do not fill in a
before/after number until real trials have been run.

## Setup

You need `results_real/multiqc/star_salmon/multiqc_report_data/multiqc_data.json`
(or any other real run's `multiqc_data.json`) and, for condition B, a built
`flowcast` binary.

Recruit at least 3-5 subjects who have not seen this specific run's data
before. Fewer than that and individual variance will dominate whatever
number comes out — say so if you report a result from fewer subjects.

## Conditions

Each subject does **both** conditions, on **different** runs (to avoid a
practice-effect confound — doing condition A on run X and condition B on
run Y, then swap which run maps to which condition for the next subject).

**Condition A — manual review.** Subject is given the raw
`multiqc_data.json` (or the human-readable MultiQC HTML report, your
choice — pick one and hold it constant across subjects) and asked: "Which
sample, if any, looks like an outlier on alignment quality, and why?"
Start the timer when they open the file; stop it when they name a sample
(or say "none") and give a reason.

**Condition B — FlowCast-assisted.** Subject runs
`./flowcast -trace <trace> -multiqc <multiqc_data.json> -reasoning REASONING.md`
and is asked the same question, now allowed to use FlowCast's output.
Start the timer when they run the command; stop it when they answer.

## Recording

Run `scripts/time_trial.sh <subject_id> <condition: manual|flowcast>` for
each trial — it starts a stopwatch on launch and appends one row to
`user_testing_results.csv` (`subject,condition,seconds,timestamp`) when you
press Enter to stop it.

## Reporting

Only report a "manual review took X minutes, FlowCast-assisted took Y
minutes" number after this has actually been run, and say how many
subjects and trials it's based on — n=3 and n=30 are not the same claim,
and the difference matters to anyone reading the number.
