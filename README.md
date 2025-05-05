# Red Panda

**Goals:**

1. [PDF](github.com/pdfcpu/pdfcpu) --> use this libray.
2. Performance: The current implementation processes files sequentially. For production use with large files or many users, consider:

   - Implementing worker pools

   - Adding file size limits

   - Setting timeout limits

3. _Security_: Always validate file contents, not just extensions, to prevent malicious uploads.
4. More formats support.
