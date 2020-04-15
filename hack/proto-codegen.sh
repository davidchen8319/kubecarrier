#!/usr/bin/env bash

# Copyright 2019 The KubeCarrier Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#!/usr/bin/env bash

set -eu

# As per official docs
# https://grpc-ecosystem.github.io/grpc-gateway/docs/usage.html

# Add protoc and protoc-gen-go tools to PATH
export PATH=${PWD}/bin:$PATH
PROJECT=$PWD
GOPATH=$(go env GOPATH)

PBUFS=(
  pkg/apis/apiserver/v1alpha1
)

# change into each protobuf directory
for pkg in ${PBUFS} ; do
  abs_path=${PROJECT}/${pkg}
  ts_root=${PROJECT}/pkg/web
  echo Generating from '*.proto' in $abs_path
  # web files due to node_modules dependency resolution should be
  protoc \
    --go_out=plugins=grpc:${abs_path}  \
    --grpc-gateway_out=logtostderr=true:${abs_path} \
    --swagger_out=logtostderr=true:${abs_path} \
    --js_out=import_style=commonjs:${ts_root} \
    --grpc-web_out=import_style=typescript,mode=grpcwebtext:${ts_root} \
    -I${PROJECT}/bin/protoc-bin/include \
    -I${GOPATH}/pkg/mod/github.com/grpc-ecosystem/grpc-gateway@v1.14.3/third_party/googleapis \
    -I/usr/include \
    -I=${abs_path} \
    $(find ${abs_path} -type f -name '*.proto')
  for x in $(find ${abs_path} -type f -name '*pb*.go'); do
    echo $x
    cat hack/boilerplate/boilerplate.generatego.txt | sed s/YEAR/$(date +%Y)/ | cat - $x > $x.tmp
    mv $x.tmp $x
    goimports -local github.com/kubermatic -w $x
  done
  pre-commit run -a pretty-format-json || true

  for ts in $(find "${ts_root}" -type f -name '*.ts' -or -name '*.js' -maxdepth 1); do
  echo "fixing ${ts}"
# It's not used within ts/js and it's hell to import properly...
ed "${ts}" <<EOF || true
g/google_api_annotations/d
w
EOF
  done
done