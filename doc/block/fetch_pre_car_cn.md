# 通过cid获取pre-car文件的block数据

1. **登陆测试服获取并保存您的explorer-token**
    - 访问测试服务器并登陆 [https://storage-test.titannet.io/](https://storage-test.titannet.io/). 并获取您的 `explorer-token`. ![Alt text](explorer_token.png)
    
2. **通过curl post create_asset接口预保存文件并获取上传地址**
    - 使用 curl 发送 POST 请求，预保存文件并获取上传地址：
    ```
    curl 'https://storage-test-api.titannet.io/api/v1/storage/create_asset' \
        -H 'accept: */*' \
        -H 'content-type: application/json' \
        -H 'jwtauthorization: Bearer <YOUR_EXPLORER_TOKEN>' \
        -H 'lang: en' \
        --data-raw '{"asset_name":"<YOUR_ASSET_NAME>","asset_type":"file","asset_size":<YOUR_ASSET_SIZE>,"group_id":0,"asset_cid":"<YOUR_ASSET_CID>","extra_id":"","need_trace":false,"area_id":[]}'
    ```
- 注意: 
    - <YOUR_EXPLORER_TOKEN> 代表上一步您登陆获取的token
    - <YOUR_ASSET_NAME> 代表您上传文件的名字
    - <YOUR_ASSET_SIZE> 代表您上传文件的大小（字节）
    - <YOUR_ASSET_CID> 代表您上传文件的根cid

- 返回的结果将包含 CandidateAddr 和 Token：
    ![Alt text](create_asset_result.png)

3. **通过curl post upload接口上传文件**
   - 复制并保存返回的 `upload_url` 地址。
   - 使用 curl post 命令将您的文件上传到该地址，例如：
    ```plaintext
    curl -X POST '<YOUR_UPLOAD_URL>' \
        -H 'Authorization: Bearer <YOUR_UPLOAD_TOKEN>' \
        -F 'file=@<YOUR_CAR_FILE_PATH>'
    ```    
- 注意: 
    - <YOUR_UPLOAD_TOKEN> 来自于上一步的返回的 Token
    - <YOUR_UPLOAD_URL> 来自于上一步返回的CandidateAddr
    - <YOUR_CAR_FILE_PATH> 来自于您本地的car文件路径

4. **获取下载链接**
   - 文件上传完成后，刷新页面, 打开浏览器的调试控制台（F12）。
   - 找到您刚上传的文件. 点击文件的下载按钮，并观察 `/api/v1/storage/share_asset` 请求的响应。 ![Alt text](share_asset.png)
   - 提取返回的下载链接，格式如下：
     ```plaintext
     https://<node-id-address>/ipfs/<YOUR-CID-HERE>?token=<YOUR-TOKEN>&filename=<YOUR-FILENAME>
     ```

5. **获取文件的所有 CID 列表**
   - 若要获取该文件下所有子级的 CID 列表，将上述链接修改为以下格式：
     ```plaintext
     https://<node-id-address>/ipfs/<YOUR-CID-HERE>?token=<YOUR-TOKEN>&format=refs
     ```
   - 访问该链接后，您将获取到包含所有子级 CID 的数组列表。![Alt text](blocks.png)

6. **获取指定 CID 的块数据**
   - 若要获取特定 CID 的块数据，将链接修改为以下格式：
     ```plaintext
     https://<node-id-address>/ipfs/<SUB-CID-YOU-PICK>?token=<YOUR-TOKEN>&format=raw
     ```
   - 访问该链接后，您将获取到所选 CID 块的具体数据。
