#!/usr/bin/env bash
bazel run gazelle -- update-repos -from_file=go.mod -prune
