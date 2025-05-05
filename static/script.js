document.addEventListener('DOMContentLoaded', () => {
    const form = document.getElementById('uploadForm');
    const fileInput = document.getElementById('fileInput');
    const progressContainer = document.getElementById('progressContainer');
    const progressBar = document.getElementById('progressBar');
    const progressText = document.getElementById('progressText');
    const statusMessage = document.getElementById('statusMessage');
    const convertButton = document.getElementById('convertButton');
    const formatSelect = document.getElementById('formatSelect');

    let fileID = null;
    let progressInterval = null;


    form.addEventListener('submit', async (e) => {
        e.preventDefault();

        if (!fileInput.files.length) {
            statusMessage.textContent = 'Please select your file';
            statusMessage.style.color = 'red';
            return
        }

        convertButton.disabled = true;
        statusMessage.textContent = 'Starting conversion....';
        statusMessage.style.color = 'inherit';

        progressContainer.style.display = 'flex';
        progressBar.style.width = '0%';
        progressText.textContent = '0%';

        fileID = Date.now().toString();

        const format = formatSelect.value;
        const formData = new FormData(form);
        formData.append('fileID', fileID);

        progressInterval = setInterval(() => {
            fetchProgress(fileID);
        }, 500);

        try {
            const response = await fetch('/convert', {
                method: "POST",
                body: formData
            });
            clearInterval(progressInterval);

            if (response.ok) {
                const filename = fileInput.files[0].name.replace(/\.[^/.]+$/, "") + "." + format;
                const blob = await response.blob();
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');

                a.href = url;
                a.download = filename;
                document.body.appendChild(a);
                a.click();
                window.URL.revokeObjectURL(url);
                a.remove();
               
                statusMessage.textContent = 'Conversion complete';
                statusMessage.style.color = 'green';
            } else {
                const error = await response.text();
                statusMessage.textContent = 'Error: ' + error;
                statusMessage.style.color = 'red';
            }
        }catch(error){
            clearInterval(progressInterval);
            statusMessage.textContent = 'Error: ' + error.message;
            statusMessage.style.color = 'red';
        }finally{
            convertButton.disabled = false;
            progressContainer.style.display = 'none';
            fileID = null;
        }
    });
    async function fetchProgress(id) {
        try {
            const response = await fetch(`/progress?id=${id}`);
            const progress = await response.text();
            
            if (progress) {
                const progressNum = parseInt(progress);
                progressBar.style.width = `${progressNum}%`;
                progressText.textContent = `${progressNum}%`;
                
                if (progressNum < 40) {
                    statusMessage.textContent = 'Uploading file...';
                } else if (progressNum < 80) {
                    statusMessage.textContent = 'Processing image...';
                } else {
                    statusMessage.textContent = 'Finalizing conversion...';
                }
            }
        } catch (error) {
            console.error('Error fetching progress:', error);
        }
    }
});
