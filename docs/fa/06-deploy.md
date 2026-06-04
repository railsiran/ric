# `ric deploy` — انتشار پروژه روی سرور

`ric deploy` کانتینری را که در آن دولوپ کرده‌اید برمی‌دارد و آن را روی یک سرور ریموت به یک سرویس production در حال اجرا تبدیل می‌کند — در صورت نیاز داکر را نصب می‌کند، credentialهای production می‌سازد، یک nginx مشترک با TLS راه می‌اندازد و اپلیکیشن را شروع می‌کند. این فقط برای اولین بار است؛ نسخه‌های بعدی از [`ric upgrade`](07-upgrade.md) رد می‌شوند.

## نحوه‌ی استفاده

```bash
# داخل پوشه‌ی پروژه
ric deploy
```

بدون flag، بدون آرگومان. همه چیز از `.ric/deploy.yml` می‌آید.

## تنظیم `.ric/deploy.yml`

این فایل را در `.ric/deploy.yml` در ریشه‌ی پروژه بسازید. مثال حداقلی:

```yaml
host: 198.51.100.42
user: root
domain: shop.example.com
ssl_dir: /etc/letsencrypt/live/shop.example.com
```

مثال کامل با همه‌ی گزینه‌ها:

```yaml
# --- اتصال SSH ---
host: 198.51.100.42             # اجباری: IP یا hostname
user: root                      # اجباری: کاربر SSH (root یا دارای sudo بدون رمز)
port: 2222                      # اختیاری: پیش‌فرض 22
ssh_key: ~/.ssh/id_ed25519      # اختیاری: حذف کنید تا با رمز عبور وارد شوید

# --- اپلیکیشن / TLS ---
domain: shop.example.com            # اجباری: server_name nginx
ssl_dir: ~/certs/shop.example.com   # اجباری: پوشه‌ی محلی شامل fullchain.pem + privkey.pem

# --- آینه‌ی رجیستری (اختیاری) ---
registry_mirror: dockerhub.example.com   # اگر VPS شما نمی‌تواند مستقیما به Docker Hub وصل شود
```

### مرجع فیلدها

| فیلد | اجباری | معنی |
|-------|----------|---------|
| `host` | بله | IP یا hostname سرور |
| `user` | بله | کاربر SSH — باید root باشد یا sudo بدون رمز داشته باشد (ric در صورت نیاز یک‌بار خودش راه می‌اندازد) |
| `ssh_key` | خیر | مسیر کلید خصوصی SSH. حذف کنید تا با رمز عبور وارد شوید — یک‌بار از شما پرسیده می‌شود |
| `port` | خیر | پورت SSH (پیش‌فرض 22) |
| `domain` | بله | دامنه‌ای که اپلیکیشن از طریق آن سرو می‌شود؛ به `server_name` در nginx می‌رود |
| `ssl_dir` | بله | پوشه‌ی **محلی** شامل گواهی به صورت `fullchain.pem` + `privkey.pem` |
| `registry_mirror` | خیر | آینه‌ی pull-through مربوط به Docker Hub که توسط daemon استفاده می‌شود (وقتی Docker Hub فیلتر یا کند است مفید است) |

`~` در `ssh_key` و `ssl_dir` گسترش داده می‌شود.

## کاری که `ric deploy` انجام می‌دهد

۱. **اعتبارسنجی محلی** — `.ric/deploy.yml` را می‌خواند، وجود فایل‌های SSL را بررسی می‌کند، و در حال اجرا بودن کانتینر دولوپمنت را تایید می‌کند.
۲. **ساخت ایمیج** — `RAILS_ENV=production bin/rails assets:precompile`، سپس `docker commit` و `docker save` به یک `.tar` با آدرس‌دهی محتوایی (`<sha256>.tar`). ایمیج موقت محلی پاک می‌شود.
۳. **باز کردن اتصال master SSH** — multiplex می‌شود تا فقط یک‌بار (در صورت وجود) رمز عبور وارد کنید.
۴. **تشخیص دسترسی روی ریموت** — اگر کاربر SSH شما root نیست و sudo بدون رمز ندارد، ric یک‌بار رمز sudo را می‌پرسد و `/etc/sudoers.d/ric-<user>` را می‌نویسد تا deployهای بعدی بدون مداخله انجام شوند.
۵. **نصب داکر** روی سرور در صورت عدم وجود.
۶. **پیکربندی آینه‌ی رجیستری** در `/etc/docker/daemon.json` و اعتبارسنجی آن با pull کردن `hello-world`.
۷. **آماده‌سازی layout** در `/var/lib/ric/<project>/` (credentials، storage، images) و `/var/lib/ric/_shared/` مشترک (sites-enabled، ssl، ric-nginx).
۸. **آپلود فایل tar ایمیج**، گواهی‌های SSL، و یک site config تولیدشده‌ی nginx.
۹. **تولید credentialهای Rails production** روی سرور (`config/credentials/production.key` + `production.yml.enc`) و نگه‌داشتن دائمی آن‌ها؛ master key را به `.ric/keys/production.key` محلی (mode 0600) برمی‌گرداند تا بعدا قابل بازیابی باشد.
۱۰. **اجرای `db:prepare`** (دیتابیس‌های Solid Trifecta را زیر mount دائمی `storage/` می‌سازد، migrate می‌کند و seed می‌زند).
۱۱. **شروع کانتینر اپلیکیشن** با `bin/rails server -b 0.0.0.0 -p 3000` و `SOLID_QUEUE_IN_PUMA=true` (worker صف داخل پروسه‌ی web اجرا می‌شود).
۱۲. **شروع `ric-nginx` مشترک** اگر در حال اجرا نیست، bind به 80/443، و سپس reload کردن آن برای دریافت site config شما.
۱۳. **نوشتن `current_sha`** به عنوان نشانگر موفقیت.

سطر پایانی: `Deployed <name> @ <short_sha> to <host>. Live at https://<domain>`.

## بعد از deploy

- `.ric/keys/production.key` روی سیستم شما وجود دارد — **`.ric/keys/` را به `.gitignore` خود اضافه کنید**.
- layout ریموت زیر `/var/lib/ric/<name>/` و `/var/lib/ric/_shared/` همان چیزی است که اجراهای بعدی `ric upgrade` روی آن بنا می‌کنند.
- REPL production می‌خواهید؟ `ric console --remote` ([`ric console`](04-console.md) را ببینید).
- نسخه‌ی جدید می‌خواهید بفرستید؟ [`ric upgrade`](07-upgrade.md).

## چند پروژه روی یک سرور

`ric deploy` دقیقا برای این طراحی شده است. اولین پروژه‌ای که deploy می‌شود یک `ric-nginx` تک‌نسخه‌ای راه می‌اندازد و به 80/443 bind می‌کند؛ پروژه‌های بعدی فقط یک site config زیر `/var/lib/ric/_shared/nginx/sites-enabled/<name>.conf` می‌گذارند و `nginx -s reload` آن را برمی‌دارد. مسیریابی بر اساس `server_name` انجام می‌شود (همان `domain` که برای هر پروژه پیکربندی کرده‌اید).

## اجرای مجدد deploy

اگر `ric deploy` را اجرا کنید و deploy قبلی کامل شده باشد (نشانگر `current_sha` وجود داشته باشد)، رد می‌کند و شما را به `ric upgrade` ارجاع می‌دهد. این عمدی است — `deploy` آماده‌سازی است؛ `upgrade` انتشار.

اگر `ric deploy` قبلی *در میانه راه crash کرده باشد*، حالت ناقص خودکار در اجرای بعدی تشخیص داده و پاک می‌شود.

## رفع اشکال

- **«the registry mirror is not serving …»** — مقدار mirror در `.ric/deploy.yml` شما اشتباه است (معمولا scheme ای مثل `https://` یا یک مسیر در آن گنجانده شده). `/etc/docker/daemon.json` روی سرور را بررسی کنید.
- **کانتینر اپلیکیشن در حلقه‌ی restart** — `ssh user@host docker logs <name>`. علل رایج: gem از قلم افتاده، خطای migration، مشکل master key.
- **nginx در حلقه‌ی restart** — `ssh user@host docker logs ric-nginx`. معمولا مسیر/فرمت گواهی SSL یا یک typo در site config تولیدشده.
