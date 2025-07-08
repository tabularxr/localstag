#!/bin/bash

# Cleanup script to revert folder to initial state
# Removes all generated files and directories

echo "Cleaning up localstag directory..."

# Remove build artifacts
rm -rf bin/
rm -f local-pipeline
rm -f *.exe

# Remove runtime data
rm -rf stag-data/
rm -rf stag-data
rm -rf test-data/
rm -rf *-data/
rm -f *.db
rm -f *.bolt

# Remove logs
rm -f *.log
rm -f *.pid

# Remove temporary files
rm -rf tmp/
rm -rf temp/
rm -rf .tmp/

# Remove Go specific
rm -rf vendor/

# Remove IDE files
rm -rf .vscode/
rm -rf .idea/
rm -f *.swp
rm -f *.swo
rm -f *~

# Remove OS files
rm -f .DS_Store
rm -f Thumbs.db

# Remove test outputs
rm -rf test-output/
rm -f coverage.out
rm -f *.test

# Remove development artifacts
rm -f debug
rm -f profile.out

echo "Cleanup complete!"