// script.js
document.getElementById('uploadForm').addEventListener('submit', function(e) {
  e.preventDefault();

  let formData = new FormData(this);

  fetch('/convert', {
      method: 'POST',
      body: formData
  })
  .then(response => response.json())
  .then(data => {
      if (data.download_url) {
          document.getElementById('result').style.display = 'block';
          document.getElementById('downloadLink').href = data.download_url;
      } else {
          alert('Conversion failed!');
      }
  })
  .catch(error => {
      console.error('Error:', error);
      alert('An error occurred while processing your request.');
  });
});
