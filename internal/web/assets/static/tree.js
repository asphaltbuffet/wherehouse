// Behaviors for the inventory tree (a drawer on narrow screens), add-modal,
// and detail pane.
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
    // onModalFormAfterRequest opens the dialog once the fresh form is in.
    htmx.ajax('GET', url, { target: '#add-modal-body', swap: 'innerHTML' });
  }

  function clearTreeSelection() {
    document.querySelectorAll('.tree-item').forEach(function (e) {
      e.classList.remove('selected');
      var ti = e.closest('[role="treeitem"]');
      if (ti) ti.setAttribute('aria-selected', 'false');
    });
  }

  // Narrow viewports show the tree as a drawer (ADR 0037). Keep this query in
  // step with the max-width breakpoint in app.css.
  var narrow = window.matchMedia('(max-width: 48rem)');

  // setDrawerInert keeps Tab out of the panels behind an open drawer.
  function setDrawerInert(on) {
    document.getElementById('detail-panel').inert = on;
    var footer = document.querySelector('.app-footer');
    if (footer) footer.inert = on;
  }

  function openTree() {
    document.body.classList.add('tree-open');
    document.getElementById('tree-toggle').setAttribute('aria-expanded', 'true');
    setDrawerInert(true);
    focusItem(document.querySelector('#tree-root-list .tree-item.selected') ||
      document.querySelector('#tree-root-list .tree-item'));
  }

  // closeTree hides the drawer and moves focus to focusEl (null leaves focus alone).
  function closeTree(focusEl) {
    if (!document.body.classList.contains('tree-open')) return;
    document.body.classList.remove('tree-open');
    document.getElementById('tree-toggle').setAttribute('aria-expanded', 'false');
    setDrawerInert(false);
    if (focusEl) focusEl.focus();
  }

  function onTreeToggleClick(e) {
    var toggle = document.getElementById('tree-toggle');
    if (e.target.closest && e.target.closest('#tree-toggle')) {
      if (document.body.classList.contains('tree-open')) {
        closeTree(toggle);
      } else {
        openTree();
      }
      return;
    }
    if (e.target.id === 'tree-backdrop') closeTree(toggle);
  }

  function onDrawerKeydown(e) {
    if (e.key !== 'Escape' || !document.body.classList.contains('tree-open')) return;
    var dialog = document.getElementById('add-modal');
    if (dialog && dialog.open) return; // Esc belongs to the modal first
    e.preventDefault();
    closeTree(document.getElementById('tree-toggle'));
  }

  // A header search swaps results into the detail panel, so get the drawer out
  // of the way; focus stays in the search box so typing continues.
  function onSearchBeforeRequest(e) {
    if (narrow.matches && e.target.closest && e.target.closest('.app-search')) closeTree(null);
  }

  // liveChevron matches a chevron that is showing (a leaf's is hidden, and a
  // click there is a plain row click).
  var liveChevron = '.chevron:not(.chevron-hidden)';

  // A chevron click only expands or collapses and a + click only opens the
  // add modal; neither may also load the row's detail (which would change the
  // selection and close the drawer).
  function onTreeBeforeRequest(e) {
    var ev = e.detail && e.detail.requestConfig && e.detail.requestConfig.triggeringEvent;
    var control = ev && ev.target && ev.target.closest &&
      ev.target.closest(liveChevron + ', .add-btn');
    if (control && e.target.classList && e.target.classList.contains('tree-item')) e.preventDefault();
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
  //   .add-btn → open add-child modal
  //   chevron  → expand/collapse (onTreeBeforeRequest cancels the detail load)
  // Any other click on a .tree-item only selects: the row's hx-get loads the
  // detail and syncTreeSelection highlights the row (#364).
  function onTreeClick(e) {
    var addBtn = e.target.closest('.add-btn');
    if (addBtn) {
      // Listeners share document.body (see onTreeEvent), so stop the later
      // ones (e.g. onDetailAddClick) from also seeing this click.
      e.stopImmediatePropagation();
      openAddModal(addBtn.dataset.parentId || '');
      return;
    }
    var chevron = e.target.closest(liveChevron);
    if (chevron) toggleNode(chevron.closest('.tree-item'));
  }

  // After a tree row loads its detail, close the drawer on narrow screens
  // (syncTreeSelection does the highlighting).
  function onTreeAfterRequest(e) {
    var item = e.target.closest && e.target.closest('.tree-item');
    if (!item) return;
    if (narrow.matches) closeTree(document.getElementById('detail-panel'));
  }

  // shownId is the entity the detail panel last showed. revealId is set when
  // that changes and cleared once the entity's ancestors are all expanded, so
  // a branch the user collapses afterwards stays collapsed.
  var shownId = '';
  var revealId = '';

  // expandAncestors expands the shown entity's ancestors (data-ancestor-ids,
  // root first). It returns true while a branch is still loading; that
  // branch's settle runs syncTreeSelection again to continue down the path.
  function expandAncestors(section) {
    var ids = (section.dataset.ancestorIds || '').split(' ').filter(Boolean);
    for (var i = 0; i < ids.length; i++) {
      var item = document.querySelector('#tree-root-list .tree-item[data-entity-id="' + ids[i] + '"]');
      if (!item) return false;
      if (!item.dataset.expanded) {
        toggleNode(item); // fetches the children (a no-op on a leaf)
        return !!item.dataset.expanded;
      }
      var list = document.getElementById('tree-children-' + ids[i]);
      if (!list) return false;
      if (list.classList.contains('collapsed')) toggleNode(item);
      if (!list.children.length) return true; // fetched, not yet swapped in
    }
    return false;
  }

  // Highlight the tree row of the entity the detail panel shows (none on
  // home), expanding its ancestors when the shown entity changes. Runs after
  // every settle, so it also catches navigation from breadcrumbs, the
  // children list and search, and tree nodes that load after the detail did
  // (a borrow's new node, an expanded branch).
  function syncTreeSelection() {
    var section = document.querySelector('#detail-panel #entity-detail');
    var id = section ? section.dataset.entityId : '';
    if (id !== shownId) {
      shownId = id;
      revealId = id;
    }
    clearTreeSelection();
    if (!id) return;
    var reveal = revealId === id;
    if (reveal && expandAncestors(section)) return;
    revealId = '';
    var item = document.querySelector('#tree-root-list .tree-item[data-entity-id="' + id + '"]');
    if (!item) return;
    item.classList.add('selected');
    var li = item.closest('[role="treeitem"]');
    if (li) li.setAttribute('aria-selected', 'true');
    // The narrow drawer scrolls on open instead (openTree focuses the row).
    if (reveal && !narrow.matches) item.scrollIntoView({ block: 'nearest' });
  }

  // Arrows expand/collapse and move focus; Enter/Space select (WAI-ARIA tree).
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
          toggleNode(div);
        } else if (expanded === 'true') {
          var childLi = li && li.querySelector('ul [role="treeitem"]');
          focusItem(childLi);
        }
        break;
      case 'ArrowLeft':
        e.preventDefault();
        if (li && li.getAttribute('aria-expanded') === 'true') {
          toggleNode(div);
        } else {
          var parentLi = li && li.parentElement && li.parentElement.closest('[role="treeitem"]');
          focusItem(parentLi);
        }
        break;
      case 'Enter':
      case ' ':
        e.preventDefault();
        div.click(); // selects: the row's hx-get loads its detail
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

  // After any GET swaps a form into #add-modal-body (add, borrow), show the
  // dialog. Opening only after the swap means the previous form never
  // flashes. htmx dispatches afterRequest on the requesting element, so
  // match on the swap target in e.detail instead.
  function onModalFormAfterRequest(e) {
    if (!e.detail || !e.detail.target || e.detail.target.id !== 'add-modal-body') return;
    var dialog = document.getElementById('add-modal');
    if (dialog && !dialog.open) dialog.showModal();
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

  // Parent pickers (borrow, move, reports): picking a type-ahead result fills
  // the hidden input and the visible label of its own picker — the nearest
  // [data-parent-picker], else the form (a form may hold several pickers).
  // The clear button writes its data-parent-value ('' for borrow's root,
  // 'root' for move's) and shows its data-empty-label, default '(root)'.
  function onParentPick(e) {
    var pick = e.target.closest && e.target.closest('[data-action="pick-parent"]');
    var clear = e.target.closest && e.target.closest('[data-action="clear-parent"]');
    if (!pick && !clear) return;
    var form = e.target.closest('form');
    if (!form) return;
    var scope = e.target.closest('[data-parent-picker]') || form;
    scope.querySelector('[data-parent-input]').value = pick ? pick.dataset.parentId : (clear.dataset.parentValue || '');
    scope.querySelector('[data-parent-label]').textContent = pick ? pick.dataset.parentPath : (clear.dataset.emptyLabel || '(root)');
    scope.querySelector('[data-parent-options]').innerHTML = '';
    scope.querySelector('[data-parent-search]').value = '';
  }

  // Enter in a parent search box must not submit its form.
  function onParentSearchKeydown(e) {
    if (e.key === 'Enter' && e.target.matches && e.target.matches('[data-parent-search]')) e.preventDefault();
  }

  // Esc in a [data-esc-cancel] form clicks its Cancel button. An htmx
  // keyup[...] trigger filter needs eval, which CSP forbids.
  function onEscCancelKeydown(e) {
    if (e.key !== 'Escape' || !e.target.closest) return;
    var form = e.target.closest('form[data-esc-cancel]');
    var cancel = form && form.querySelector('[data-action="cancel"]');
    if (!cancel) return;
    e.preventDefault();
    cancel.click();
  }

  // Reports' Print button (CSP forbids inline onclick).
  function onPrintClick(e) {
    if (e.target.closest && e.target.closest('[data-action="print"]')) window.print();
  }

  // treeNodeName is the display name a tree <li> shows.
  function treeNodeName(li) {
    var label = li.querySelector(':scope > .tree-item > [id^="tree-label-"]');
    return label ? label.textContent : '';
  }

  // placeSorted moves entityId's tree node to where the server would list it
  // among its siblings: by display name, then entity ID, compared by code
  // unit to match SQLite's BINARY collation (so not localeCompare).
  function placeSorted(entityId) {
    var li = document.getElementById('tree-node-' + entityId);
    if (!li || !li.parentElement) return;
    var name = treeNodeName(li);
    var next = Array.from(li.parentElement.children).find(function (sib) {
      if (sib === li) return false;
      var sibName = treeNodeName(sib);
      return sibName > name || (sibName === name && sib.id > li.id);
    });
    li.parentElement.insertBefore(li, next || null);
  }

  // Insert a node that joined parentId's list (add, borrow, move) in sorted
  // position when that list is already loaded (root, or a parent expanded at
  // least once). A never-expanded parent only needs its chevron revealed;
  // children load on expand.
  function onTreeNodeAttached(e) {
    var parentId = e.detail.parentId;
    var listId = parentId ? 'tree-children-' + parentId : 'tree-root-list';
    if (!document.getElementById(listId)) return;
    if (parentId) {
      var parentItem = document.querySelector('.tree-item[data-entity-id="' + parentId + '"]');
      if (parentItem && !parentItem.dataset.expanded) {
        var li = parentItem.closest('li');
        if (li && li.getAttribute('aria-expanded') === null) li.setAttribute('aria-expanded', 'false');
        var ch = parentItem.querySelector('.chevron');
        if (ch) ch.classList.remove('chevron-hidden');
        return;
      }
    }
    var entityId = e.detail.entityId;
    htmx.ajax('GET', '/entities/' + entityId + '/tree-node', {
      target: '#' + listId,
      swap: 'beforeend',
    }).then(function () { placeSorted(entityId); });
  }

  // After a rename (once the OOB label swap is in): re-sort the node.
  function onTreeNodeRenamed(e) {
    placeSorted(e.detail.entityId);
  }

  // After an add or borrow: close the modal, then insert the new node.
  function onEntityCreated(e) {
    var dialog = document.getElementById('add-modal');
    if (dialog) dialog.close();
    if (narrow.matches) closeTree(document.getElementById('detail-panel')); // show the result
    onTreeNodeAttached(e);
  }

  // After a node leaves its parent's list (return, move; later remove): if
  // the parent has no live children left, reset it to the leaf shape that
  // tree_node renders. onEntityCreated then treats it as never-expanded, so a
  // later borrow reveals the chevron and children load on expand.
  function onTreeNodeDetached(e) {
    var parentId = e.detail.parentId;
    if (!parentId || e.detail.parentHasChildren) return;
    var item = document.querySelector('.tree-item[data-entity-id="' + parentId + '"]');
    if (!item) return;
    var li = item.closest('li');
    if (li) li.removeAttribute('aria-expanded');
    delete item.dataset.expanded;
    var ch = item.querySelector('.chevron');
    if (ch) {
      ch.classList.add('chevron-hidden');
      ch.classList.remove('open');
    }
    var list = document.getElementById('tree-children-' + parentId);
    if (list) list.classList.remove('collapsed');
  }

  // onTreeEvent delegates an event from document.body to handler(root, e) when
  // it originates inside the tree. Binding to #tree-root-list directly would
  // be lost whenever htmx restores history, which replaces body's contents.
  function onTreeEvent(handler) {
    return function (e) {
      var root = e.target.closest && e.target.closest('#tree-root-list');
      if (root) handler(root, e);
    };
  }

  document.addEventListener('DOMContentLoaded', function () {
    document.body.addEventListener('click', onTreeToggleClick);
    document.body.addEventListener('keydown', onDrawerKeydown);
    document.body.addEventListener('htmx:beforeRequest', onSearchBeforeRequest);
    document.body.addEventListener('htmx:afterSettle', syncTreeSelection);
    syncTreeSelection(); // full-page load of /entities/{id}
    document.body.addEventListener('htmx:beforeRequest', onTreeEvent(function (root, e) { onTreeBeforeRequest(e); }));
    narrow.addEventListener('change', function (e) { if (!e.matches) closeTree(null); });

    // Tree listeners first: onTreeClick stops later body listeners for .add-btn.
    document.body.addEventListener('click', onTreeEvent(function (root, e) { onTreeClick(e); }));
    document.body.addEventListener('htmx:afterRequest', onTreeEvent(function (root, e) { onTreeAfterRequest(e); }));
    document.body.addEventListener('keydown', onTreeEvent(onTreeKeydown));

    document.body.addEventListener('htmx:afterRequest', onModalFormAfterRequest);
    document.body.addEventListener('click', onDetailAddClick);
    document.body.addEventListener('click', onAddModalCancel);
    document.body.addEventListener('click', onParentPick);
    document.body.addEventListener('click', onPrintClick);
    document.body.addEventListener('keydown', onParentSearchKeydown);
    document.body.addEventListener('keydown', onEscCancelKeydown);
    document.body.addEventListener('entity-created', onEntityCreated);
    document.body.addEventListener('tree-node-detached', onTreeNodeDetached);
    document.body.addEventListener('tree-node-attached', onTreeNodeAttached);
    document.body.addEventListener('tree-node-renamed', onTreeNodeRenamed);
  });
}());
