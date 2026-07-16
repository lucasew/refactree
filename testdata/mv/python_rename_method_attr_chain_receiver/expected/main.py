class Box:
    def assist(self):
        return 1

    def stay(self):
        return 2


def use(obj):
    return obj.box.assist() + obj.box.stay()
