# Copyright 2026 Code Capsules

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at

#     http://www.apache.org/licenses/LICENSE-2.0

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

PLATFORMS := linux_amd64
include ../../build/makelib/common.mk

IMAGE = $(BUILD_REGISTRY)/mysql-operator-orchestrator-$(ARCH)
CACHE_IMAGES = $(IMAGE)
include ../../build/makelib/image.mk

img.build:
	@$(INFO) docker build $(IMAGE) $(IMAGE_PLATFORM)
	@cp -La . $(IMAGE_TEMP_DIR)
	@cp ../../LICENSE ../../NOTICE $(IMAGE_TEMP_DIR)/
	@mkdir -p $(IMAGE_TEMP_DIR)/rootfs/usr/local/bin
	@cp $(OUTPUT_DIR)/bin/linux_$(ARCH)/orc-helper $(IMAGE_TEMP_DIR)/rootfs/usr/local/bin/orc-helper
	@docker buildx build $(BUILD_ARGS) \
		--platform $(IMAGE_PLATFORM) \
		-t $(IMAGE) \
		--build-arg ARCH=$(ARCH) \
		$(IMAGE_TEMP_DIR)
	@$(OK) docker build $(IMAGE)
