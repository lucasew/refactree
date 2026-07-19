from types import SimpleNamespace
import types
import copy


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_vars_sub():
    da = SimpleNamespace(k=A())
    db = SimpleNamespace(k=B())
    return vars(da)["k"].execute() + vars(db)["k"].run()


def use_vars_get():
    da = SimpleNamespace(k=A())
    db = SimpleNamespace(k=B())
    return vars(da).get("k").execute() + vars(db).get("k").run()


def use_vars_assign():
    da = SimpleNamespace(k=A())
    db = SimpleNamespace(k=B())
    va = vars(da)
    vb = vars(db)
    return va["k"].execute() + vb["k"].run()


def use_vars_assign_get():
    da = SimpleNamespace(k=A())
    db = SimpleNamespace(k=B())
    va = vars(da)
    vb = vars(db)
    return va.get("k").execute() + vb.get("k").run()


def use_copy_vars_sub():
    da = SimpleNamespace(k=A())
    db = SimpleNamespace(k=B())
    return copy.copy(vars(da)["k"]).execute() + copy.copy(vars(db)["k"]).run()


def use_copy_vars_assign():
    da = SimpleNamespace(k=A())
    db = SimpleNamespace(k=B())
    xa = copy.copy(vars(da)["k"])
    xb = copy.copy(vars(db)["k"])
    return xa.execute() + xb.run()


def use_dunder_sub():
    da = SimpleNamespace(k=A())
    db = SimpleNamespace(k=B())
    return da.__dict__["k"].execute() + db.__dict__["k"].run()


def use_dunder_assign():
    da = SimpleNamespace(k=A())
    db = SimpleNamespace(k=B())
    ua = da.__dict__
    ub = db.__dict__
    return ua["k"].execute() + ub["k"].run()


def use_walrus_vars():
    da = SimpleNamespace(k=A())
    db = SimpleNamespace(k=B())
    if (wa := vars(da)) and (wb := vars(db)):
        return wa["k"].execute() + wb["k"].run()
    return 0


def use_values_for():
    da = SimpleNamespace(k=A())
    db = SimpleNamespace(k=B())
    s = 0
    for x in vars(da).values():
        s += x.execute()
    for y in vars(db).values():
        s += y.run()
    return s


def use_values_assign_for():
    da = SimpleNamespace(k=A())
    db = SimpleNamespace(k=B())
    va = vars(da)
    vb = vars(db)
    s = 0
    for x in va.values():
        s += x.execute()
    for y in vb.values():
        s += y.run()
    return s


def use_types_sns_vars():
    da = types.SimpleNamespace(k=A())
    db = types.SimpleNamespace(k=B())
    ta = vars(da)
    tb = vars(db)
    return ta["k"].execute() + tb["k"].run() + copy.copy(vars(da)["k"]).execute() + copy.copy(vars(db)["k"]).run()


def use_preserves_b():
    db = SimpleNamespace(k=B())
    vb = vars(db)
    return vb["k"].run() + copy.copy(vars(db)["k"]).run()
