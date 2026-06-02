# Copyright 2026 Code Capsules
# SPDX-License-Identifier: Apache-2.0
#
# Copy the mysql-operator binary into a thin image
# The image is pinned to the nonroot tag
FROM gcr.io/distroless/base-debian11@sha256:e76722f06f7c15e0076072fb02782ec59923b0d658b8a3d80bb79deaee6fb44d

# Apache 2.0 redistribution (object form): include LICENSE and NOTICE for recipients.
COPY LICENSE NOTICE /licenses/

COPY rootfs /
ENTRYPOINT ["/mysql-operator"]
CMD ["help"]
