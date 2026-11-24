.PHONY: spread
spread:
	[ -d .dev/spread-plus ] || git clone --branch devel --depth 1 https://github.com/nulltrek/spread-plus .dev/spread-plus
	git -C .dev/spread-plus pull origin devel
	cd .dev/spread-plus && make build
	cp .dev/spread-plus/spread-plus bin/spread
