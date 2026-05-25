// Keyboard navigation for the inventory tree (WAI-ARIA tree pattern).
// Handles ArrowUp/Down (move focus), ArrowRight (expand/focus first child),
// ArrowLeft (collapse/focus parent), Enter/Space (activate), Home/End.
//
// Structure: <li role="treeitem"> wraps <div class="tree-item" tabindex="0">.
// Focus lives on the .tree-item div; ARIA state lives on the parent <li>.
(function () {
  'use strict';

  // Returns all visible focusable tree-item divs (not the <li> role elements).
  function visibleItems(root) {
    return Array.from(root.querySelectorAll('.tree-item')).filter(
      el => el.offsetParent !== null
    );
  }

  // Returns the <li role="treeitem"> ancestor for a .tree-item div.
  function treeitemLi(el) {
    return el.closest('[role="treeitem"]');
  }

  function activate(div) {
    div.click();
  }

  // Focus a .tree-item div; if given a <li role="treeitem">, focus its inner div.
  function focusItem(el) {
    if (!el) return;
    var div = el.classList.contains('tree-item') ? el : el.querySelector('.tree-item');
    if (div) div.focus();
  }

  document.addEventListener('DOMContentLoaded', function () {
    var root = document.getElementById('tree-root-list');
    if (!root) return;

    root.addEventListener('keydown', function (e) {
      var div = e.target.closest('.tree-item');
      if (!div) return;

      var items = visibleItems(root);
      var idx   = items.indexOf(div);
      var li    = treeitemLi(div);

      switch (e.key) {
        case 'ArrowDown':
          e.preventDefault();
          if (idx < items.length - 1) items[idx + 1].focus();
          break;
        case 'ArrowUp':
          e.preventDefault();
          if (idx > 0) items[idx - 1].focus();
          break;
        case 'ArrowRight':
          e.preventDefault();
          var expanded = li && li.getAttribute('aria-expanded');
          if (expanded === 'false') {
            activate(div);
          } else if (expanded === 'true') {
            var childLi = li && li.querySelector('ul [role="treeitem"]');
            focusItem(childLi);
          }
          break;
        case 'ArrowLeft':
          e.preventDefault();
          if (li && li.getAttribute('aria-expanded') === 'true') {
            activate(div);
          } else {
            var itemLi = li;
            var parentLi = itemLi && itemLi.parentElement && itemLi.parentElement.closest('[role="treeitem"]');
            focusItem(parentLi);
          }
          break;
        case 'Enter':
        case ' ':
          e.preventDefault();
          activate(div);
          break;
        case 'Home':
          e.preventDefault();
          if (items.length) items[0].focus();
          break;
        case 'End':
          e.preventDefault();
          if (items.length) items[items.length - 1].focus();
          break;
      }
    });
  });
}());
