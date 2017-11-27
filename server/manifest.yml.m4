---
applications:
- name: websox-server        
  memory: 32M
  disk_quota: 32M
  instances: 1
  timeout: 80
  command: ./server
  buildpack: https://github.com/cloudfoundry/binary-buildpack.git
  env:
        uaa_client_id: UAA_CLIENT_ID
        uaa_client_secret: UAA_CLIENT_SECRET
        uaa_url: UAA_URL
        refresh_period: 10m
