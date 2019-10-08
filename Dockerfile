FROM golang:1.13

LABEL "com.github.actions.name"="Manifold Auto-Tagging Bot"
LABEL "com.github.actions.description"="Manifold Auto-Tagging Bot"
LABEL "com.github.actions.icon"="thumbs-up"
LABEL "com.github.actions.color"="green"

WORKDIR /go/src/app
COPY . .
RUN GO111MODULE=on go build -o autotagger .
ENTRYPOINT ["/go/src/app/autotagger"]
