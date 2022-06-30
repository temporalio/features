# Running activities concurrently
Activities may be run concurrently.

Each feature workflow in this folder should demonstrate running 2 or more activities
concurrently, and collect their results.

# Detailed spec
* When workflow code starts more than one activity at the same time, the next WFT
  completion it sends will contain Schedule Activity commands for each activity
* Server will then issue activity tasks for all of those activities
* As each activity completes, corresponding events are added to history
* When the worker receives workflow task(s) with those events, the activities
  are resolved correspondingly