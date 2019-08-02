#!/bin/sh
echo "# HELP script_has_timeout Whether this script is run with a timeout."
echo "# TYPE script_has_timeout gauge"
if [ -z "$SCRIPT_TIMEOUT" ]; then
	echo "script_has_timeout{} 0"
	exit 0
fi
echo "script_has_timeout{} 1"

echo "# HELP script_timeout_seconds Timeout of the script in seconds"
echo "# TYPE script_timeout_seconds gauge"
echo "script_timeout_seconds{} $SCRIPT_TIMEOUT"

echo "# HELP script_deadline_seconds Unix timestamp when the timeout will expire"
echo "# TYPE script_deadline_seconds gauge"
echo "script_deadline_seconds{} $SCRIPT_DEADLINE"

echo "# HELP script_timeout_enforced Whether or not script_exporter is enforcing a timeout on the script."
echo "# TYPE script_timeout_enforced gauge"
echo "script_timeout_enforced{} $SCRIPT_TIMEOUT_ENFORCED"
exit 0
