class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_match_unannotated_map():
    da = {"k": [A()]}
    db = {"k": [B()]}
    match da:
        case {"k": [xa, *_]}:
            r = xa.execute()
    match db:
        case {"k": [xb, *_]}:
            r += xb.run()
    return r


def use_match_unannotated_map_as():
    da = {"k": [A()]}
    db = {"k": [B()]}
    match da:
        case {"k": [xa as x, *_]}:
            r = x.execute()
    match db:
        case {"k": [xb as y, *_]}:
            r += y.run()
    return r


def use_match_unannotated_map_row():
    da = {"k": [A()]}
    db = {"k": [B()]}
    match da:
        case {"k": row}:
            r = row[0].execute()
    match db:
        case {"k": rowb}:
            r += rowb[0].run()
    return r


def use_match_unannotated_map_row_as():
    da = {"k": [A()]}
    db = {"k": [B()]}
    match da:
        case {"k": row as r}:
            x = r[0].execute()
    match db:
        case {"k": rowb as s}:
            x += s[0].run()
    return x


def use_match_unannotated_map_tuple():
    da = {"k": (A(),)}
    db = {"k": (B(),)}
    match da:
        case {"k": (ta, *_)}:
            r = ta.execute()
    match db:
        case {"k": (tb, *_)}:
            r += tb.run()
    return r


def use_sub_unannotated_map():
    da = {"k": [A()]}
    db = {"k": [B()]}
    return da["k"][0].execute() + db["k"][0].run()


def use_var_unannotated_map():
    da = {"k": [A()]}
    db = {"k": [B()]}
    ga = da["k"]
    gb = db["k"]
    return ga[0].execute() + gb[0].run()


def use_for_unannotated_map():
    da = {"k": [A()]}
    db = {"k": [B()]}
    n = 0
    for a in da["k"]:
        n += a.execute()
    for b in db["k"]:
        n += b.run()
    return n


def use_values_for_unannotated_map():
    da = {"k": [A()]}
    db = {"k": [B()]}
    n = 0
    for ga in da.values():
        n += ga[0].execute()
    for gb in db.values():
        n += gb[0].run()
    return n


def use_multi_key_unannotated_map():
    da = {"k": [A()], "m": [A()]}
    db = {"k": [B()], "m": [B()]}
    return da["k"][0].execute() + da["m"][0].execute() + db["k"][0].run() + db["m"][0].run()


def use_preserves_b():
    db = {"k": [B()]}
    match db:
        case {"k": [xb, *_]}:
            return xb.run()
