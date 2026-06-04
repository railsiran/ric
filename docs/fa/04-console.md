# `ric console` — کنسول تعاملی Rails

`ric console` شما را وارد `bin/rails console` داخل کانتینر پروژه می‌کند. همان Ruby، همان gemها، همان دیتابیسی که در کانتینر هست — فقط با یک REPL تعاملی روی آن.

## نحوه‌ی استفاده

```bash
# کانتینر محلی دولوپمنت
ric console

# کنسول production روی سرور deploy شده
ric console --remote
```

از داخل پوشه‌ی پروژه اجرا کنید. نام کانتینر همان نام پوشه است (که `ric new` ساخته است).

## حالت محلی (بدون flag)

یک کنسول Rails داخل کانتینر دولوپمنت محلی باز می‌کند. این کنسول به دیتابیس SQLite دولوپمنت زیر `storage/` متصل می‌شود. با `exit` یا `Ctrl+D` خارج شوید.

```bash
$ ric console
Opening Rails console for myapp...
Loading development environment (Rails 8.0.x)
irb(main):001> User.count
=> 0
```

## حالت ریموت (`--remote`)

یک کنسول Rails **داخل کانتینر production روی سرور deploy شده‌ی شما** باز می‌کند. از همان اتصال SSH ای استفاده می‌کند که ric برای deploy استفاده می‌کند، با multiplex برای سرعت بیشتر. `host`، `user`، `port` و `key` (یا رمز) را از `.ric/deploy.yml` می‌خواند.

```bash
$ ric console --remote
Opening production Rails console for myapp on 198.51.100.42...
Loading production environment (Rails 8.0.x)
irb(main):001> User.last.email
=> "real-customer@example.com"
```

### پیش‌نیازها

- پروژه باید از قبل با [`ric deploy`](06-deploy.md) deploy شده باشد.
- `.ric/deploy.yml` باید در پوشه‌ی فعلی موجود باشد.
- کاربر SSH شما روی سرور باید root باشد یا sudo بدون رمز داشته باشد (ric این را در اولین deploy برای شما تنظیم می‌کند).

### مراقب باشید

کنسول ریموت با **داده‌های واقعی production** کار می‌کند. مثل ssh کردن به production با آن رفتار کنید:

- از عملیات مخرب پرهیز کنید مگر اینکه مطمئن باشید.
- `Model.delete_all` برگشت‌ناپذیر است — راه بازگشتی وجود ندارد.
- کوئری‌های طولانی یک اتصال از pool اپلیکیشن شما را اشغال می‌کنند.

وقتی کارتان تمام شد، `exit` یا `Ctrl+D`. اتصال master SSH هنگام خروج خودش جمع می‌شود، پس هیچ session باقی‌مانده‌ای روی سرور نمی‌ماند.

## مرحله‌ی بعد

- به جای یک session تعاملی، یک دستور Rails یک‌باره اجرا کنید: [`ric rails`](05-rails.md).
- deploy یا upgrade کنید: [`ric deploy`](06-deploy.md)، [`ric upgrade`](07-upgrade.md).
