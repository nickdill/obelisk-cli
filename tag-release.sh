#!/usr/bin/env bash
set -e

latest=$(git tag --sort=-version:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1)

if [ -z "$latest" ]; then
  next="v0.1.0"
else
  IFS='.' read -r major minor patch <<< "${latest#v}"
  next="v${major}.${minor}.$((patch + 1))"
fi

echo "Creating tag $next..."
git tag "$next"
echo "Done. Push with: git push origin $next"
