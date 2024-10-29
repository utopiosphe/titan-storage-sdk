# 文件上传与 CID 列表、数据获取指南

1. **上传文件**
   - 访问测试服务器 [https://storage-test.titannet.io/](https://storage-test.titannet.io/)，登录并上传您需要演示的文件。

2. **获取下载链接**
   - 文件上传完成后，打开浏览器的调试控制台（F12）。
   - 点击文件的下载按钮，并观察 `/api/v1/storage/share_asset` 请求的响应。 ![Alt text](share_asset.png)
   - 提取返回的下载链接，格式如下：
     ```plaintext
     https://<node-id-address>/ipfs/<YOUR-CID-HERE>?token=<YOUR-TOKEN>&filename=<YOUR-FILENAME>
     ```

3. **获取文件的所有 CID 列表**
   - 若要获取该文件下所有子级的 CID 列表，将上述链接修改为以下格式：
     ```plaintext
     https://<node-id-address>/ipfs/<YOUR-CID-HERE>?token=<YOUR-TOKEN>&format=refs
     ```
   - 访问该链接后，您将获取到包含所有子级 CID 的数组列表。![Alt text](blocks.png)

4. **获取指定 CID 的块数据**
   - 若要获取特定 CID 的块数据，将链接修改为以下格式：
     ```plaintext
     https://<node-id-address>/ipfs/<SUB-CID-YOU-PICK>?token=<YOUR-TOKEN>&format=raw
     ```
   - 访问该链接后，您将获取到所选 CID 块的具体数据。
