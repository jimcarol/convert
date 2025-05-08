// script.js
const fileOptionsMap = {
    pdf: ['docx'],
    doc: ['pdf'],          // PDF => show DOCX
    docx: ['pdf'],         // DOCX => show PDF
    png: ['webp'],         // PNG => show WEBP
    jpg: ['webp'],        // JPG => show WEBP
    jpeg: ['webp'],        // JPG => show WEBP
};
document.getElementById('file').addEventListener('change', function(e) {
    const file = e.target.files[0];  // Get the first file selected
    const fileType = file ? file.type : '';  // MIME type
    const fileExtension = file ? file.name.split('.').pop().toLowerCase() : ''; 

    const select = document.getElementById('target');
    // Clear existing options
    select.innerHTML = '<option value="">Select an target type</option>';

    // Handle both file extension and MIME type
    const options = fileOptionsMap[fileExtension] || fileOptionsMap[fileType] || [];
    options.forEach(option => {
        select.innerHTML += `<option value="${option}">Convert to ${option.toUpperCase()}</option>`;
      });
  
    // If no valid options, show a fallback message
    if (options.length === 0) {
        select.innerHTML += '<option value="">No valid conversion options available</option>';
    }
})

document.getElementById('uploadForm').addEventListener('submit', function(e) {
  e.preventDefault();
  clearState()
  let formData = new FormData(this);

  if (!formData.get('target')) {
    insertResult(false, {}, 'Please select target type.')

    return 
  }

  // 获取上传的文件
  let file = formData.get('file');
  // 限制文件大小为 5MB
  const maxSize = 5 * 1024 * 1024; // 5MB
  if (file && file.size > maxSize) {
    insertResult(false, {}, 'File size cannot exceed 5MB');
    return;
  }

  fetch('/convert', {
      method: 'POST',
      body: formData
  })
  .then(response => {
    const contentType = response.headers.get('content-type');
    if (contentType && contentType.includes("json")) {
        return response.json()
    } else {
        return response.text()
    }
  })
  .then(data => {
      if (data.download_url) {
        insertResult(true, data, "")
      } else {
        //   alert('Conversion failed!');
          insertResult(false, data, data || 'Conversion failed!')
      }
  })
  .catch(error => {
      console.error('Error:', error);
    //   alert('An error occurred while processing your request.');
      insertResult(false, {}, 'An error occurred while processing your request.')
  });
});

async function copyToClipboard(text) {
  try {
    await navigator.clipboard.writeText(text);
  } catch (error) {
    console.error(error.message);
  }
}

function insertResult(bol, data, message) {
    resultContainer = document.getElementById('result');
    const fileItem = document.createElement('div');
    fileItem.className = 'file-item';
    if (bol) {
        fileItem.innerHTML = `
            <div class="file-info">
                <i class="fas fa-file-pdf"></i>
                <a href="${data.download_url}" target="_blank">${data.download_url.split('/').slice(-1)}</a>
            </div>
            <div>
                <button class="copy-btn" onclick="copyToClipboard('${location.origin}${data.download_url}')">Copy Link</button>
                <button class="download-btn" onclick="window.location.href='${data.download_url}'">Download</button>
            </div>
        `;
    } else {
        fileItem.innerHTML = `
            <i class="fas fa-exclamation-triangle text-danger" style="margin-right: 10px;"></i>
            <span class="file-error text-danger" style="margin-left: 10px; flex: 1; color: red;">${message}</span>
        `;
    }
    
    // Add the list item to the file list
    resultContainer.appendChild(fileItem);
    resultContainer.style.display = 'block'
}

function clearState() {
    resultContainer = document.getElementById('result');
    resultContainer.innerHTML = "";
    resultContainer.style.display = 'none';
}
