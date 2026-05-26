// Behaviors for the inventory tree, add-modal, and detail pane.
//
// Everything is wired by event delegation against stable container IDs so the
// HTML templates can stay free of inline event handlers (Content-Security-Policy
// friendly).
//
// Tree structure: <li role="treeitem"> wraps <div class="tree-item" tabindex="0">.
// Focus lives on the .tree-item div; ARIA state lives on the parent <li>.
(function () {
  'use strict';

  function visibleItems(root) {
    return Array.from(root.querySelectorAll('.tree-item')).filter(
      el => el.offsetParent !== null
    );
  }

  function treeitemLi(el) {
    return el.closest('[role="treeitem"]');
  }

  function focusItem(el) {
    if (!el) return;
    var div = el.classList.contains('tree-item') ? el : el.querySelector('.tree-item');
    if (div) div.focus();
  }

  function openAddModal(parentId) {
    var url = parentId ? '/entities/' + parentId + '/add' : '/entities/add';
    htmx.ajax('GET', url, { target: '#add-modal-body', swap: 'innerHTML' });
    document.getElementById('add-modal').showModal();
  }

  function clearTreeSelection() {
    document.querySelectorAll('.tree-item').forEach(function (e) {
      e.classList.remove('selected');
      var ti = e.closest('[role="treeitem"]');
      if (ti) ti.setAttribute('aria-selected', 'false');
    });
  }

  function toggleNode(el) {
    var li = el.closest('li');
    var chevron = el.querySelector('.chevron');
    var entityId = el.dataset.entityId;
    if (!entityId) return;
    var childrenList = document.getElementById('tree-children-' + entityId);
    if (!el.dataset.expanded && li.getAttribute('aria-expanded') !== null) {
      htmx.ajax('GET', '/tree/' + entityId + '/children', {
        target: '#tree-children-' + entityId,
        swap: 'innerHTML',
      });
      el.dataset.expanded = '1';
      li.setAttribute('aria-expanded', 'true');
      if (chevron) {
        chevron.classList.remove('chevron-hidden');
        chevron.classList.add('open');
      }
      if (childrenList) childrenList.classList.remove('collapsed');
    } else if (el.dataset.expanded) {
      var isCollapsed = childrenList && childrenList.classList.toggle('collapsed');
      if (chevron) chevron.classList.toggle('open', !isCollapsed);
      li.setAttribute('aria-expanded', isCollapsed ? 'false' : 'true');
    }
  }

  // Handle clicks anywhere in the tree:
  //   .add-btn  → open add-child modal
  //   .tree-item → toggle expand/collapse (selection is set after the htmx request)
  function onTreeClick(e) {
    var addBtn = e.target.closest('.add-btn');
    if (addBtn) {
      e.stopPropagation();
      openAddModal(addBtn.dataset.parentId || '');
      return;
    }
    var item = e.target.closest('.tree-item');
    if (item) {
      toggleNode(item);
    }
  }

  // After any htmx swap originating inside the tree, highlight the
  // .tree-item that issued the request.
  function onTreeAfterRequest(e) {
    var item = e.target.closest && e.target.closest('.tree-item');
    if (!item) return;
    clearTreeSelection();
    item.classList.add('selected');
    var li = item.closest('[role="treeitem"]');
    if (li) li.setAttribute('aria-selected', 'true');
  }

  function onTreeKeydown(root, e) {
    var div = e.target.closest('.tree-item');
    if (!div) return;
    var items = visibleItems(root);
    var idx = items.indexOf(div);
    var li = treeitemLi(div);
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
          div.click();
        } else if (expanded === 'true') {
          var childLi = li && li.querySelector('ul [role="treeitem"]');
          focusItem(childLi);
        }
        break;
      case 'ArrowLeft':
        e.preventDefault();
        if (li && li.getAttribute('aria-expanded') === 'true') {
          div.click();
        } else {
          var parentLi = li && li.parentElement && li.parentElement.closest('[role="treeitem"]');
          focusItem(parentLi);
        }
        break;
      case 'Enter':
      case ' ':
        e.preventDefault();
        div.click();
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
  }

  // After the add-form POST completes successfully, close the modal,
  // reset the form, and reveal the chevron on a parent that previously
  // had no children.
  function onAddFormAfterRequest(e) {
    if (!e.detail || !e.detail.successful) return;
    var form = e.target.closest && e.target.closest('form');
    if (!form || !form.matches('#add-modal-body form')) return;
    document.getElementById('add-modal').close();
    form.reset();
    var targetSelector = form.getAttribute('hx-target');
    var target = targetSelector ? document.querySelector(targetSelector) : null;
    if (target && target.children.length === 1) {
      var li = target.closest('li');
      if (li) {
        li.setAttribute('aria-expanded', 'false');
        var item = li.querySelector('.tree-item');
        if (item) {
          var ch = item.querySelector('.chevron');
          if (ch) ch.classList.remove('chevron-hidden');
        }
      }
    }
  }

  // After the "+ Add root" GET fetches the form into #add-modal-body,
  // show the dialog.
  function onAddRootAfterRequest(e) {
    if (e.target && e.target.id === 'add-modal-body') {
      document.getElementById('add-modal').showModal();
    }
  }

  // Add child button inside the detail pane action menu.
  function onDetailAddClick(e) {
    var btn = e.target.closest && e.target.closest('[data-action="add-child"]');
    if (!btn) return;
    openAddModal(btn.dataset.parentId || '');
  }

  // "Cancel" button inside any add-modal form.
  function onAddModalCancel(e) {
    var btn = e.target.closest && e.target.closest('[data-action="close-add-modal"]');
    if (!btn) return;
    var dialog = document.getElementById('add-modal');
    if (dialog) dialog.close();
  }

  document.addEventListener('DOMContentLoaded', function () {
    var root = document.getElementById('tree-root-list');
    if (root) {
      root.addEventListener('click', onTreeClick);
      root.addEventListener('htmx:afterRequest', onTreeAfterRequest);
      root.addEventListener('keydown', function (e) { onTreeKeydown(root, e); });
    }

    document.body.addEventListener('htmx:afterRequest', onAddFormAfterRequest);
    document.body.addEventListener('htmx:afterRequest', onAddRootAfterRequest);
    document.body.addEventListener('click', onDetailAddClick);
    document.body.addEventListener('click', onAddModalCancel);
  });
}());
