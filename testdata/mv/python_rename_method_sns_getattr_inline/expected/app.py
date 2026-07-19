from types import SimpleNamespace
import types


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_getattr():
    return (
        getattr(SimpleNamespace(k=A()), "k").execute()
        + getattr(SimpleNamespace(k=B()), "k").run()
    )


def use_getattr_types():
    return (
        getattr(types.SimpleNamespace(k=A()), "k").execute()
        + getattr(types.SimpleNamespace(k=B()), "k").run()
    )


def use_getattr_assign():
    xa = getattr(SimpleNamespace(k=A()), "k")
    xb = getattr(SimpleNamespace(k=B()), "k")
    return xa.execute() + xb.run()


def use_getattr_local_still():
    da = SimpleNamespace(k=A())
    db = SimpleNamespace(k=B())
    return getattr(da, "k").execute() + getattr(db, "k").run()


def use_preserves_b():
    return getattr(SimpleNamespace(k=B()), "k").run()
