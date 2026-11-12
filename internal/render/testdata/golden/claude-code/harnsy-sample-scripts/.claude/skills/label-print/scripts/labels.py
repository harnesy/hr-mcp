#!/usr/bin/env python3
"""Print one label line per shelf mark."""
import sys

for mark in open(sys.argv[1]):
    print("[ " + mark.strip() + " ]")
