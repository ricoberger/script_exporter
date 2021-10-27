#! /bin/bash

# scrape_configs:
#   - job_name: 'script_args'
#     metrics_path: /probe
#     params:
#       script: [args]
#       params: ["arg3,arg4"]
#       arg3: [test3]
#       arg4: [test4]
#     static_configs:
#       - targets: ["localhost:9469"]

echo "script_with_arguments{arg1=\"$1\", arg2=\"$2\", arg3=\"$3\", arg4=\"$4\"} 1"
