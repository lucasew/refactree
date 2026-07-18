from types import SimpleNamespace
import types


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_sns_attr():
    da = SimpleNamespace(k=A())
    db = SimpleNamespace(k=B())
    return da.k.execute() + db.k.run()


def use_sns_var():
    da = SimpleNamespace(k=A())
    db = SimpleNamespace(k=B())
    xa = da.k
    xb = db.k
    return xa.execute() + xb.run()


def use_sns_multi():
    da = SimpleNamespace(k=A(), m=A())
    db = SimpleNamespace(k=B(), m=B())
    return da.k.execute() + da.m.execute() + db.k.run() + db.m.run()


def use_types_sns():
    da = types.SimpleNamespace(k=A())
    db = types.SimpleNamespace(k=B())
    return da.k.execute() + db.k.run()


def use_preserves_b():
    db = SimpleNamespace(k=B())
    return db.k.run()
