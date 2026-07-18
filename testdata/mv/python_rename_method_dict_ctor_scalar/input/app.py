class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_dict_kw_sub():
    da = dict(k=A())
    db = dict(k=B())
    return da["k"].run() + db["k"].run()


def use_dict_kw_var():
    da = dict(k=A())
    db = dict(k=B())
    xa = da["k"]
    xb = db["k"]
    return xa.run() + xb.run()


def use_dict_kw_values():
    da = dict(k=A())
    db = dict(k=B())
    n = 0
    for a in da.values():
        n += a.run()
    for b in db.values():
        n += b.run()
    return n


def use_dict_kw_multi():
    da = dict(k=A(), m=A())
    db = dict(k=B(), m=B())
    return da["k"].run() + da["m"].run() + db["k"].run() + db["m"].run()


def use_dict_pairs():
    da = dict([("k", A())])
    db = dict([("k", B())])
    return da["k"].run() + db["k"].run()


def use_dict_from_literal():
    da = dict({"k": A()})
    db = dict({"k": B()})
    return da["k"].run() + db["k"].run()


def use_dict_kw_match():
    da = dict(k=A())
    db = dict(k=B())
    match da:
        case {"k": xa}:
            r = xa.run()
    match db:
        case {"k": xb}:
            r += xb.run()
    return r


def use_preserves_b():
    db = dict(k=B())
    return db["k"].run()
