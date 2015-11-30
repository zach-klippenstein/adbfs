#!/bin/sh
set -x

HELPER_DIR=./travis_test_helpers
mkdir "$HELPER_DIR"

# Create a fake fusermount so fuse package doesn't crash on init.
cat >"$HELPER_DIR/fusermount" <<EOF
#!/bin/sh
# Do nothing.
EOF
chmod +x "$HELPER_DIR/fusermount"

export PATH="$HELPER_DIR:$PATH"

set +x
