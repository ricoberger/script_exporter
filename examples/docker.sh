#! /bin/bash

# Run the following command to start a docker container using the i386/busybox image.
# docker run --name busybox -it --rm i386/busybox

result="$(docker exec -i busybox ls -la | wc -l)"
echo "$result"
echo "number_of_files $result"
