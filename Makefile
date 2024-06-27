.PHONY: run build

run:
	docker rm rtsp_redirect_resolver; docker build --tag rtsp_redirect_resolver . && docker run --name rtsp_redirect_resolver rtsp_redirect_resolver $(args)
build:
	docker rm rtsp_redirect_resolver; docker build --tag rtsp_redirect_resolver . && docker run -d --name rtsp_redirect_resolver rtsp_redirect_resolver http 1 && docker cp rtsp_redirect_resolver:/app/build/rtsp_redirect_resolver ./build && docker stop rtsp_redirect_resolver