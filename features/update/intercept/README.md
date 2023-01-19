# Update Calls Can Be Intercepted

Interceptors in sdk-go can intercept and modify update requests

# Detailed spec

Basically just that interceptors exist and work. The test here modifies the
arguments to an update on its way into the workflow and checks that that
modification is reflected in the response.

