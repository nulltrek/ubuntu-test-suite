.PHONY: spread
spread:
	[ -d .dev/spread-plus ] || git clone --branch devel --depth 1 https://github.com/nulltrek/spread-plus .dev/spread-plus
	git -C .dev/spread-plus pull origin devel
	cd .dev/spread-plus && make build
	cp .dev/spread-plus/spread-plus bin/spread

.PHONY: hwrap
hwrap:
	[ -d .dev/hwrap ] || git clone --branch framework --depth 1 https://git.launchpad.net/~nulltrek/+git/hwrap .dev/hwrap
	git -C .dev/hwrap pull origin framework
	cd .dev/hwrap && make build
	cp .dev/hwrap/hwrap bin/hwrap
