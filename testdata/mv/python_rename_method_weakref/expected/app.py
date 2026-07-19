import weakref


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_ref(a: A, b: B):
    ra = weakref.ref(a)
    rb = weakref.ref(b)
    return ra().execute() + rb().run()


def use_ref_inline(a: A, b: B):
    return weakref.ref(a)().execute() + weakref.ref(b)().run()


def use_proxy(a: A, b: B):
    pa = weakref.proxy(a)
    pb = weakref.proxy(b)
    return pa.execute() + pb.run()


def use_proxy_inline(a: A, b: B):
    return weakref.proxy(a).execute() + weakref.proxy(b).run()
