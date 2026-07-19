from dataclasses import dataclass, replace
import weakref


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


class BoxA:
    def __init__(self, a: A):
        self.a = a

    def get(self) -> A:
        return self.a


class BoxB:
    def __init__(self, b: B):
        self.b = b

    def get(self) -> B:
        return self.b


@dataclass
class WrapA:
    a: A


@dataclass
class WrapB:
    b: B


def use_replace(ba: BoxA, bb: BoxB):
    return replace(WrapA(A()), a=ba.get()).a.execute() + replace(
        WrapB(B()), b=bb.get()
    ).b.run()


def use_replace_assign(ba: BoxA, bb: BoxB):
    xa = replace(WrapA(A()), a=ba.get()).a
    xb = replace(WrapB(B()), b=bb.get()).b
    return xa.execute() + xb.run()


def use_proxy(ba: BoxA, bb: BoxB):
    return weakref.proxy(ba.get()).execute() + weakref.proxy(bb.get()).run()


def use_proxy_assign(ba: BoxA, bb: BoxB):
    xa = weakref.proxy(ba.get())
    xb = weakref.proxy(bb.get())
    return xa.execute() + xb.run()


def use_class():
    return (
        replace(WrapA(A()), a=A()).a.execute()
        + replace(WrapB(B()), b=B()).b.run()
        + weakref.proxy(A()).execute()
        + weakref.proxy(B()).run()
    )


def use_preserves_b(bb: BoxB):
    return (
        replace(WrapB(B()), b=bb.get()).b.run()
        + weakref.proxy(bb.get()).run()
        + weakref.proxy(B()).run()
    )
