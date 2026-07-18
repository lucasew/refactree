class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_lod_sub():
    la = [{"k": A()}]
    lb = [{"k": B()}]
    return la[0]["k"].run() + lb[0]["k"].run()


def use_lod_var():
    la = [{"k": A()}]
    lb = [{"k": B()}]
    da = la[0]
    db = lb[0]
    return da["k"].run() + db["k"].run()


def use_lod_for():
    la = [{"k": A()}]
    lb = [{"k": B()}]
    n = 0
    for da in la:
        n += da["k"].run()
    for db in lb:
        n += db["k"].run()
    return n


def use_lod_values():
    la = [{"k": A()}]
    lb = [{"k": B()}]
    n = 0
    for a in la[0].values():
        n += a.run()
    for b in lb[0].values():
        n += b.run()
    return n


def use_dod_sub():
    da = {"outer": {"k": A()}}
    db = {"outer": {"k": B()}}
    return da["outer"]["k"].run() + db["outer"]["k"].run()


def use_dod_var():
    da = {"outer": {"k": A()}}
    db = {"outer": {"k": B()}}
    ia = da["outer"]
    ib = db["outer"]
    return ia["k"].run() + ib["k"].run()


def use_scalar_sub():
    da = {"k": A()}
    db = {"k": B()}
    return da["k"].run() + db["k"].run()


def use_scalar_values():
    da = {"k": A()}
    db = {"k": B()}
    n = 0
    for a in da.values():
        n += a.run()
    for b in db.values():
        n += b.run()
    return n


def use_preserves_b():
    lb = [{"k": B()}]
    return lb[0]["k"].run()
