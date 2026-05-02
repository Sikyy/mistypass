# MistyPass — Kepatuhan UU PDP Indonesia

> UU No.27 Tahun 2022 tentang Pelindungan Data Pribadi (PDP)
> Berlaku penuh: Oktober 2024
> Dokumen ini menjelaskan bagaimana MistyPass memenuhi persyaratan UU PDP

---

## 1. Ringkasan Kepatuhan

MistyPass adalah sistem kontrol akses (access control) berbasis cloud yang dapat di-deploy secara **self-hosted** di infrastruktur lokal Indonesia. Model self-hosted memberikan keunggulan kepatuhan yang signifikan dibandingkan SaaS asing karena **data tidak pernah meninggalkan yurisdiksi Indonesia**.

| Prinsip UU PDP | Status MistyPass | Implementasi |
|----------------|------------------|-------------|
| Dasar hukum pemrosesan | Memenuhi | Consent-based + legitimate interest (keamanan akses) |
| Pembatasan tujuan | Memenuhi | Data hanya digunakan untuk kontrol akses dan audit |
| Minimalisasi data | Memenuhi | Hanya mengumpulkan data yang diperlukan untuk identifikasi dan otorisasi |
| Akurasi data | Memenuhi | CRUD lengkap untuk pembaruan data pengguna |
| Pembatasan penyimpanan | Memenuhi | Configurable retention policy (audit log, snapshot, event) |
| Integritas dan kerahasiaan | Memenuhi | Enkripsi AES-256-GCM, TLS 1.3, HMAC audit chain |
| Akuntabilitas | Memenuhi | Immutable audit trail dengan HMAC chain |

---

## 2. Keunggulan Model Self-Hosted

### 2.1 Kedaulatan Data (Data Sovereignty)

| Aspek | MistyPass (Self-Hosted) | SaaS Asing (Contoh: Kisi) |
|-------|------------------------|--------------------------|
| Lokasi server | Indonesia (pilihan pelanggan) | AS / Uni Eropa (GCP) |
| Transfer lintas batas | **Tidak ada** | Ya — perlu consent tambahan |
| Yurisdiksi hukum | Indonesia | AS / Uni Eropa |
| Kontrol fisik | Pelanggan | Vendor |
| Audit server | Kapan saja oleh pelanggan | Terbatas oleh SLA vendor |

### 2.2 Kontrol Penuh atas Data Pribadi

Sebagai operator self-hosted, organisasi pelanggan:
- Memiliki kontrol penuh atas database PostgreSQL
- Dapat menentukan kebijakan retensi sendiri
- Dapat menghapus data secara permanen kapan saja
- Tidak bergantung pada vendor untuk data export/deletion

---

## 3. Data Pribadi yang Diproses

### 3.1 Kategori Data

| Kategori | Contoh Data | Tujuan | Dasar Hukum |
|----------|-------------|--------|-------------|
| Identitas karyawan | Nama, email, nomor karyawan | Identifikasi untuk kontrol akses | Legitimate interest (keamanan) |
| Kredensial akses | NFC UID, kartu fisik | Otorisasi masuk gedung | Legitimate interest (keamanan) |
| Log akses | Waktu masuk/keluar, lokasi pintu | Audit keamanan | Kewajiban hukum (keamanan kerja) |
| Data pengunjung | Nama, nomor KTP, kontak | Registrasi tamu | Consent |
| Foto/video | Snapshot kamera saat akses | Keamanan fisik | Legitimate interest (keamanan) |
| Data biometrik | WebAuthn credential (passkey) | Autentikasi MFA | Consent |

### 3.2 Data yang TIDAK Dikumpulkan

MistyPass **tidak** mengumpulkan:
- Data keuangan / rekening bank
- Data kesehatan
- Data agama / etnis / politik
- Riwayat browsing / lokasi GPS terus-menerus
- Data anak di bawah umur (sistem untuk kontrol akses perusahaan)

---

## 4. Hak Subjek Data (Pasal 5-13 UU PDP)

| Hak | Cara Pemenuhan | Endpoint/Fitur |
|-----|---------------|----------------|
| Hak akses (Pasal 5) | Pengguna dapat melihat data pribadi melalui My Account | `GET /api/v1/users/me` |
| Hak koreksi (Pasal 6) | Pengguna dapat memperbarui profil | `PATCH /api/v1/users/{id}` |
| Hak hapus (Pasal 7) | Admin dapat menghapus pengguna; pengguna dapat self-delete | `DELETE /api/v1/user` |
| Hak tarik consent (Pasal 9) | Pengguna dapat menonaktifkan kredensial | `PATCH /api/v1/card_assignments/{id}/deactivate` |
| Hak portabilitas (Pasal 11) | Export data via CSV | `GET /api/v1/users?format=csv` |
| Hak ajukan keberatan (Pasal 12) | Hubungi admin organisasi | Proses manual via admin |
| Hak informasi (Pasal 13) | Kebijakan privasi organisasi | Tanggung jawab pelanggan |

### 4.1 Penghapusan Data

Saat pengguna dihapus dari sistem:
1. Data profil pengguna dihapus dari tabel `auth_users`
2. Kredensial fisik (kartu NFC) di-deaktivasi
3. Sesi aktif dicabut
4. Log audit **dipertahankan** sesuai kebijakan retensi (untuk kewajiban keamanan)

> **Catatan**: Log audit yang mengandung data pribadi minimal (user_id, waktu, aksi) dipertahankan selama periode retensi untuk memenuhi kewajiban keamanan kerja. Ini sesuai dengan Pasal 15 UU PDP yang mengecualikan pemrosesan untuk kepentingan hukum.

---

## 5. Keamanan Data (Pasal 35 UU PDP)

### 5.1 Enkripsi

| Lapisan | Metode | Detail |
|---------|--------|--------|
| Data at rest (database) | AES-256-GCM | Via `crypto.Vault` untuk kredensial sensitif |
| Data at rest (kamera) | AES-256-GCM | `CAMERA_VAULT_MASTER_KEY` |
| Data in transit | TLS 1.3 | HTTPS untuk semua komunikasi |
| Gateway ↔ Cloud | TLS + Device Token | Certificate pinning |
| Password | bcrypt | Cost factor 12 |

### 5.2 Kontrol Akses

| Mekanisme | Detail |
|-----------|--------|
| Autentikasi | JWT + Refresh Token + WebAuthn (FIDO2 Passkey) |
| MFA | TOTP + Recovery Codes |
| RBAC | 5 level: super_admin, tenant_admin, operator, building_admin, resident |
| Multi-tenant | Isolasi data ketat per `tenant_id` |
| Rate limiting | 600 req/min (API), 10 req/min (login) |

### 5.3 Audit Trail

MistyPass menggunakan **HMAC chain audit log** yang menjamin:
- Setiap entri audit ditandatangani secara kriptografis
- Modifikasi atau penghapusan entri akan terdeteksi (tamper-evident)
- Rantai audit dapat diverifikasi secara independen

Ini **lebih kuat** dari standar audit log yang diwajibkan UU PDP.

---

## 6. Notifikasi Pelanggaran Data (Pasal 46 UU PDP)

UU PDP mewajibkan notifikasi dalam **3 x 24 jam** setelah mengetahui pelanggaran data.

### 6.1 Deteksi

MistyPass menyediakan mekanisme deteksi:
- Alert Policy untuk event keamanan (Door Forced Open, Hardware Outage, dll)
- Audit trail tamper detection
- Gateway offline detection
- Failed authentication monitoring

### 6.2 Prosedur Notifikasi (Tanggung Jawab Pelanggan)

Karena MistyPass adalah self-hosted, prosedur notifikasi pelanggaran adalah **tanggung jawab organisasi pelanggan**:

1. **Deteksi** - Sistem MistyPass mendeteksi anomali dan mengirim alert
2. **Investigasi** - Tim keamanan pelanggan melakukan investigasi (< 24 jam)
3. **Notifikasi regulator** - Pelanggan melapor ke Lembaga PDP (< 3 x 24 jam)
4. **Notifikasi subjek data** - Pelanggan memberitahu individu yang terdampak
5. **Dokumentasi** - Catatan insiden disimpan dalam audit log

---

## 7. Penilaian Dampak (Data Protection Impact Assessment)

Untuk pemrosesan berisiko tinggi, organisasi pelanggan harus melakukan DPIA. MistyPass mendukung ini dengan:

| Komponen DPIA | Dukungan MistyPass |
|---------------|-------------------|
| Inventaris data | API endpoint list + OpenAPI spec |
| Pemetaan aliran data | Arsitektur: Gateway → Cloud → Database |
| Analisis risiko | Audit log + event monitoring |
| Mitigasi | Enkripsi, RBAC, MFA, rate limiting |
| Monitoring | Real-time event stream (SSE), alert policies |

---

## 8. Penunjukan Data Protection Officer

Sesuai Pasal 53 UU PDP, organisasi yang memproses data pribadi dalam skala besar wajib menunjuk DPO. Ini adalah **tanggung jawab organisasi pelanggan**, bukan vendor MistyPass.

MistyPass mendukung DPO dengan:
- Akses audit log yang komprehensif
- Export data untuk pelaporan
- Webhook untuk integrasi dengan sistem compliance

---

## 9. Rekomendasi untuk Pelanggan

### 9.1 Sebelum Go-Live

- [ ] Deploy MistyPass di data center Indonesia (IDC, Biznet, Telkom, dll)
- [ ] Konfigurasi `TZ=Asia/Jakarta` dan `DEFAULT_TIMEZONE=Asia/Jakarta`
- [ ] Tetapkan kebijakan retensi audit log (disarankan: 1-3 tahun)
- [ ] Buat kebijakan privasi yang mencakup pemrosesan data akses
- [ ] Siapkan prosedur notifikasi pelanggaran data (3 x 24 jam)
- [ ] Tunjuk DPO jika memproses data skala besar

### 9.2 Operasional

- [ ] Review akses pengguna secara berkala (fitur Access Rights Review)
- [ ] Monitor alert policies untuk deteksi anomali
- [ ] Backup database secara teratur
- [ ] Update MistyPass ke versi terbaru untuk patch keamanan

---

## 10. Referensi

- UU No.27 Tahun 2022 tentang Pelindungan Data Pribadi
- PP No.17 Tahun 2025 tentang Pelaksanaan UU PDP (jika sudah terbit)
- Peraturan Lembaga PDP (jika sudah terbit)
- MistyPass OpenAPI Spec: `GET /api/v1/openapi.json`
- MistyPass Credential Security Architecture: `docs/credential-security-architecture.md`
- MistyPass Hardware Integration Guide: `docs/hardware-integration-guide.md`

---

*Dokumen ini disusun sebagai panduan teknis kepatuhan dan bukan merupakan nasihat hukum. Organisasi pelanggan disarankan untuk berkonsultasi dengan penasihat hukum untuk memastikan kepatuhan penuh terhadap UU PDP.*
