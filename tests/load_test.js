import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter } from 'k6/metrics';

// Özel sayaçlar
export const successfulOrders = new Counter('successful_orders');
export const rejectedOrders = new Counter('rejected_orders');

export const options = {
  scenarios: {
    stock_rush: {
      executor: 'per-vu-iterations',
      vus: 50,              // Aynı anda 50 eşzamanlı kullanıcı
      iterations: 1,        // Her kullanıcı 1 kez istek atacak
      maxDuration: '30s',
    },
  },
};

export default function () {
  const url = 'http://localhost:8080/reserve-stock';
  
  // Test edeceğimiz ürün ID'si ve rastgele üretilen Order ID
  const payload = JSON.stringify({
    product_id: 1,
    order_id: `ORD-TEST-${__VU}-${Date.now()}`,
    quantity: 1,
    expiration_secs: 300,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  const res = http.post(url, payload, params);

  // Yanıt kontrolleri
  const isSuccess = check(res, {
    'stok rezerve edildi (200)': (r) => r.status === 200,
  });

  if (isSuccess) {
    successfulOrders.add(1);
  } else {
    rejectedOrders.add(1);
    check(res, {
      'yetersiz stok ile reddedildi': (r) => r.status !== 200,
    });
  }

  sleep(0.1);
}