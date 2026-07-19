class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_subscript_assign():
    da = {}
    db = {}
    da["k"] = A()
    db["k"] = B()
    return da["k"].execute() + db["k"].run()


def use_setdefault_stmt():
    da = {}
    db = {}
    da.setdefault("k", A())
    db.setdefault("k", B())
    return da["k"].execute() + db["k"].run()


def use_setdefault_ret():
    da = {}
    db = {}
    a = da.setdefault("k", A())
    b = db.setdefault("k", B())
    return a.execute() + b.run()


def use_setdefault_chain():
    da = {}
    db = {}
    return da.setdefault("k", A()).execute() + db.setdefault("k", B()).run()


def use_update_dict():
    da = {}
    db = {}
    da.update({"k": A()})
    db.update({"k": B()})
    return da["k"].execute() + db["k"].run()


def use_update_kw():
    da = {}
    db = {}
    da.update(k=A())
    db.update(k=B())
    return da["k"].execute() + db["k"].run()


def use_preserves_b():
    db = {}
    db["k"] = B()
    db.setdefault("j", B())
    db.update({"m": B()})
    return db["k"].run() + db["j"].run() + db["m"].run()
