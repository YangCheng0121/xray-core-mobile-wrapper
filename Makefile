ios:
	gomobile clean
	rm -rf ./build
	gomobile bind -target=ios,iossimulator -o ./build/XRayCore.xcframework github.com/YangCheng0121/xray-core-mobile-wrapper
