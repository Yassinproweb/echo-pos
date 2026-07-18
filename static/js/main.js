document.addEventListener("DOMContentLoaded", () => {
  const statusColors = {
    placed: { text: "text-pos_plc", bg: "bg-pos_plc" },
    transit: { text: "text-pos_trs", bg: "bg-pos_trs" },
    waiting: { text: "text-pos_trs", bg: "bg-pos_trs" },
    pickup: { text: "text-pos_trs", bg: "bg-pos_trs" },
    ready: { text: "text-pos_rdy", bg: "bg-pos_rdy" },
    preparing: { text: "text-pos_pdg", bg: "bg-pos_pdg" },
    canceled: { text: "text-pos_cld", bg: "bg-pos_cld" },
    taken: { text: "text-pos_dlv", bg: "bg-pos_dlv" },
    served: { text: "text-pos_dlv", bg: "bg-pos_dlv" },
    delivered: { text: "text-pos_dlv", bg: "bg-pos_dlv" },
  };

  document.querySelectorAll(".status").forEach((statusSpan) => {
    const status = statusSpan.textContent.trim().toLowerCase();
    const color = statusColors[status];
    if (!color) return;

    const card = statusSpan.closest(".snap-center");
    if (!card) return;

    // Apply color to .status
    statusSpan.classList.add(color.text);

    // Apply color to all .scolor elements inside the same card
    card.querySelectorAll(".scolor").forEach((el) => {
      el.classList.add(color.text);
    });

    // Apply bg to .sback inside the card
    card.querySelectorAll(".sback").forEach((el) => {
      el.classList.add(color.bg);
    });
  });

  const typeIcons = {
    takeaway: "hgi-packaging",
    dinein: "hgi-spoon-and-fork",
    delivery: "hgi-scooter-02",
  };

  document.querySelectorAll(".snap-center").forEach((card) => {
    const typeSpan = card.querySelector(".otype");
    if (!typeSpan) return;

    const type = typeSpan.textContent.trim().toLowerCase();
    const iconClass = typeIcons[type];
    if (!iconClass) return;

    const icon = card.querySelector(".otype-icon");
    if (icon) icon.classList.add(iconClass);
  });
});

// Returns true if the order-list modal (views/others/orders.html) is
// currently open. On pages without that modal (e.g. the main POS screen)
// this is always false, so callers fall back to the #order-panel flow.
function isOrderModalOpen() {
  const modal = document.getElementById('order-modal');
  return !!modal && modal.style.display !== 'none' && modal.style.display !== '';
}

// Re-fetches an order's receipt fragment and drops it into the order-list
// modal body in place (used after status-changing actions triggered from
// that modal, where the server endpoint's own response isn't the receipt).
function refreshModalOrder(orderId) {
  const safeId = encodeURIComponent(orderId);
  fetch(`/pos/order/${safeId}`)
    .then((r) => r.text())
    .then((html) => {
      const body = document.getElementById('order-modal-body');
      if (body) body.innerHTML = html;
    });
}

// Called the FIRST time a receipt (kitchen ticket / order-card) is printed,
// right after order creation. Placed -> Preparing. Also used to reprint the
// order-card while already Preparing (no-op status-wise).
function handlePrint(orderId, orderType) {
  const safeId = encodeURIComponent(orderId);

  window.print();

  setTimeout(() => {
    if (isOrderModalOpen()) {
      // Viewing from the orders list: update status, then refresh the
      // modal with the current receipt (not the blank order_form).
      fetch(`/pos/order/update-status/${safeId}`, { method: 'POST' })
        .then(() => refreshModalOrder(orderId));
    } else {
      // Main POS screen, right after creating an order: reset #order-panel
      // to a blank order_form for the next order.
      htmx.ajax('POST', `/pos/order/update-status/${safeId}`, {
        target: '#order-panel',
        swap: 'innerHTML'
      });
    }
  }, 1500);
}

// Called when a cashier prints the ACTUAL RECEIPT while viewing an order
// that is already Ready. This renders/prints the receipt, then ~15s after
// the print completes, auto-advances the order:
// DineIn -> Waiting, Takeaway -> PickUp, Delivery -> Transit.
function handleViewPrint(orderId) {
  const safeId = encodeURIComponent(orderId);

  if (isOrderModalOpen()) {
    fetch(`/pos/order/${safeId}/print`, { method: 'POST' })
      .then((r) => r.text())
      .then((html) => {
        const body = document.getElementById('order-modal-body');
        if (body) body.innerHTML = html;

        window.print();

        setTimeout(() => {
          fetch(`/pos/order/${safeId}/print/advance`, { method: 'POST' })
            .then(() => refreshModalOrder(orderId));
        }, 15000);
      });
    return;
  }

  htmx.ajax('POST', `/pos/order/${safeId}/print`, {
    target: '#order-panel',
    swap: 'innerHTML'
  }).then(() => {
    window.print();

    setTimeout(() => {
      htmx.ajax('POST', `/pos/order/${safeId}/print/advance`, {
        swap: 'none'
      });
    }, 15000);
  });
}

// Manual sequential status change: moves an order exactly one step forward
// or backward along its Type's status sequence (e.g. Delivery:
// Placed -> Preparing -> Ready -> Transit -> Delivered). The server
// validates the step is adjacent and rejects anything else.
function handleManualStatus(orderId, targetStatus) {
  const safeId = encodeURIComponent(orderId);
  const body = new URLSearchParams({ status: targetStatus });

  fetch(`/pos/order/${safeId}/status`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body
  })
    .then(async (r) => {
      const text = await r.text();
      if (!r.ok) {
        alert(text);
        return;
      }

      const container = isOrderModalOpen()
        ? document.getElementById('order-modal-body')
        : document.getElementById('order-panel');

      if (container) container.innerHTML = text;
    })
    .catch(() => alert('Could not update order status.'));
}
