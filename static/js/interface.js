(() => {
  const form = document.getElementById('language-form');
  if (!form) return;
  form.elements.return_to.value += location.hash;
  let dirty = false;
  // Switching language reloads the page; protect drafts in forms and rich editors.
  document.addEventListener('input', event => {
    if (!form.contains(event.target) && event.target.closest('form, [contenteditable="true"], .reader-notes')) dirty = true;
  });
  form.addEventListener('submit', event => {
    if (dirty && !confirm(window.yogilibT('Unsaved changes will be lost. Change language?'))) event.preventDefault();
  });
})();

// Accessible names for the rich editor's icon-only controls.
document.querySelectorAll('.ql-toolbar button, .ql-toolbar .ql-picker').forEach(control => {
  const labels = {bold:'Bold',italic:'Italic',underline:'Underline',strike:'Strike',color:'Color',background:'Background',align:'Align',blockquote:'Blockquote','code-block':'Code block',link:'Link',image:'Image',clean:'Clear formatting',header:'Heading',font:'Font',size:'Size'};
  let key;
  for (const name in labels) if (control.classList.contains('ql-'+name)) key = labels[name];
  if (control.classList.contains('ql-script')) key = control.value === 'sub' ? 'Subscript' : 'Superscript';
  if (control.classList.contains('ql-list')) key = {ordered:'Ordered list',bullet:'Bullet list',check:'Checklist'}[control.value];
  if (control.classList.contains('ql-indent')) key = control.value === '-1' ? 'Decrease indent' : 'Increase indent';
  if (key) { control.setAttribute('aria-label',window.yogilibT(key)); control.setAttribute('title',window.yogilibT(key)); }
});

document.querySelectorAll('.delete-form').forEach(form => {
  form.addEventListener('submit', event => {
    if (!confirm(form.dataset.confirm)) event.preventDefault();
  });
});
