#!/bin/bash
docker build --no-cache . -t registry.cn-hangzhou.aliyuncs.com/ali_eryajf/openldap:1.4.1
docker push registry.cn-hangzhou.aliyuncs.com/ali_eryajf/openldap:1.4.1