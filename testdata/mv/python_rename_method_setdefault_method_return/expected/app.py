class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


class BoxA:
    def __init__(self):
        self.a = A()

    def get(self) -> A:
        return self.a


class BoxB:
    def __init__(self):
        self.b = B()

    def get(self) -> B:
        return self.b


def use_setdefault_mr(ba: BoxA, bb: BoxB):
    da = {}
    db = {}
    return da.setdefault("k", ba.get()).execute() + db.setdefault("k", bb.get()).run()


def use_setdefault_mr_assign(ba: BoxA, bb: BoxB):
    da = {}
    db = {}
    xa = da.setdefault("k", ba.get())
    xb = db.setdefault("k", bb.get())
    return xa.execute() + xb.run()


def use_setdefault_mr_subscript(ba: BoxA, bb: BoxB):
    da = {}
    db = {}
    da.setdefault("k", ba.get())
    db.setdefault("k", bb.get())
    return da["k"].execute() + db["k"].run()


def use_update_kw_mr(ba: BoxA, bb: BoxB):
    da = {}
    db = {}
    da.update(k=ba.get())
    db.update(k=bb.get())
    return da["k"].execute() + db["k"].run()


def use_update_dict_mr(ba: BoxA, bb: BoxB):
    da = {}
    db = {}
    da.update({"k": ba.get()})
    db.update({"k": bb.get()})
    return da["k"].execute() + db["k"].run()


def use_update_pairs_mr(ba: BoxA, bb: BoxB):
    da = {}
    db = {}
    da.update([("k", ba.get())])
    db.update([("k", bb.get())])
    return da["k"].execute() + db["k"].run()


# Class regression — already worked.
def use_setdefault_class():
    da = {}
    db = {}
    return da.setdefault("k", A()).execute() + db.setdefault("k", B()).run()


def use_update_class():
    da = {}
    db = {}
    da.update(k=A())
    db.update({"k": B()})
    return da["k"].execute() + db["k"].run()


def use_preserves_b(bb: BoxB):
    db = {}
    db.update(k=bb.get())
    return db.setdefault("k", bb.get()).run() + db["k"].run()
