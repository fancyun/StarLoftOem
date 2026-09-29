<template>
  <div class="markdown-body">
    <h1>API v1 文档</h1>
    <p class="lead">
      本文档介绍星楼网络平台 API v1 的鉴权方式与接口调用方法。所有接口均使用
      <strong>API Key + HMAC-SHA256 签名</strong> 鉴权。
    </p>

    <h2>1. 基础信息</h2>
    <ul>
      <li>接口地址：<code>https://api.starloft.cn/v1</code></li>
      <li>请求格式：<code>application/json</code></li>
      <li>鉴权方式：请求头携带 API Key 与签名（见「请求鉴权」）</li>
    </ul>

    <h2>2. 调用前提（必读）</h2>
    <p>调用本 API 前，账户必须同时满足以下条件：</p>
    <ol>
      <li>
        <strong>账户已完成实名认证。</strong>实名状态（<code>realname_status</code>）按产品区分：
        人脸核验（<code>fv</code>）需<strong>企业实名</strong>（<code>realname_status=2</code>）；
        短信（<code>sms</code>）完成<strong>个人实名</strong>（<code>realname_status=1</code>）即可，
        未实名（<code>0</code>）时所有服务均不可用。请在星楼网络控制台完成对应实名认证。
      </li>
      <li>
        <strong>创建 API 密钥。</strong>在控制台「API 密钥管理」中创建，可创建多把并各自配置权限；
        密钥对（API Key + API Secret）创建后展示，API Secret 用于请求签名与回调验签。
      </li>
      <li>
        <strong>API 密钥权限覆盖所调用的接口。</strong>密钥权限范围（<code>permission</code>）为
        <code>all</code>（全部接口），或按接口逐项授权的标识集合（逗号分隔，如
        <code>fv_auth,sms_send</code>）。可授权的接口标识见各接口说明与「错误码」章节，
        在控制台「API 密钥管理」中创建或编辑密钥时勾选。
        人脸核验（fv）与短信（sms）接口权限各自独立。
      </li>
    </ol>
    <p>
      未完成对应实名、密钥权限未覆盖，或密钥无效时调用相应接口，
      将返回 <code>code: 403</code> 提示。
    </p>

    <h2>3. 请求鉴权</h2>
    <p>每个请求必须携带以下 4 个请求头：</p>
    <table>
      <thead>
        <tr>
          <th>请求头</th>
          <th>说明</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td><code>X-Api-Key</code></td>
          <td>控制台「API 密钥管理」中获取的 API Key</td>
        </tr>
        <tr>
          <td><code>X-Sign</code></td>
          <td>签名，算法见下方</td>
        </tr>
        <tr>
          <td><code>X-Sign-Version</code></td>
          <td>固定值 <code>hmac_sha256</code></td>
        </tr>
        <tr>
          <td><code>X-Timestamp</code></td>
          <td>当前 Unix 时间戳（秒），允许 ±5 分钟误差（防重放）</td>
        </tr>
      </tbody>
    </table>

    <h3>签名算法</h3>
    <p>签名为对<strong>原始请求体</strong>（POST 的 JSON 字符串，与发送内容完全一致）进行 HMAC-SHA256 运算后的小写十六进制字符串：</p>
    <pre><code>sign = hex(HMAC-SHA256(api_secret, 原始请求体))</code></pre>

    <p>PHP 示例：</p>
    <pre><code>$sign = hash_hmac('sha256', $body, $apiSecret); // 小写十六进制</code></pre>

    <h2>4. 接口列表</h2>

    <h3>本产品接口</h3>
    <ul>
      <li v-if="isFv"><a href="#fv-auth">发起人脸核验 <code>/v1/fv/auth</code> · <code>/v1/fv/self</code></a></li>
      <li v-if="isFv"><a href="#fv-result">查询核验结果 <code>/v1/fv/result</code></a></li>
      <li v-if="isFv"><a href="#fv-best-img">获取活体最佳图 <code>/v1/fv/best-img</code></a></li>
      <li v-if="isFv"><a href="#fv-media">获取认证媒体 <code>/v1/fv/media</code></a></li>
      <li v-if="isSms"><a href="#sms-sign">创建短信签名 <code>/v1/sms/signs</code></a></li>
      <li v-if="isSms"><a href="#sms-sign-get">查询与删除短信签名 <code>/v1/sms/signs/:id</code></a></li>
      <li v-if="isSms"><a href="#sms-send">短信发送 <code>/v1/sms/send</code></a></li>
      <li v-if="isSms"><a href="#sms-report">查询发送回执 <code>/v1/sms/report</code></a></li>
      <li v-if="isSms"><a href="#sms-notify">回执主动推送 <code>notify_url</code></a></li>
      <li v-if="isSms"><a href="#sms-reply">短信回复推送与查询 <code>/v1/sms/replies</code></a></li>
    </ul>

    <h3>4.4 产品与资源包</h3>
    <p>平台提供的下游能力与人脸核验子产品计费方式如下：</p>
    <table>
      <thead>
        <tr>
          <th>产品</th>
          <th>服务标识</th>
          <th>说明</th>
        </tr>
      </thead>
      <tbody>
        <tr><td>有源人脸（FV）</td><td><code>fv_auth</code></td><td>人脸 + 公安库真实身份比对</td></tr>
        <tr><td>无源人脸（FV）</td><td><code>fv_self</code></td><td>人脸与本人留底照片比对</td></tr>
      </tbody>
    </table>
    <p>
      人脸核验按子产品（<code>fv_auth</code> / <code>fv_self</code>）各自独立计费，并对应独立资源包。
      短信服务（<code>sms</code>）按<strong>短信资源包</strong>计费，先扣资源包、余额兜底；
      短信发送时通过 <code>sign_name</code> 选择签名、<code>template_id</code> 与 <code>template_params</code> 选择模板与参数，
      <code>sms_type</code> 区分验证码 / 通知短信。
    </p>

    <template v-if="isFv">
    <h3 id="fv-auth">4.5 发起人脸核验（FV）</h3>
    <p><span class="method post">POST</span><code>/v1/fv/auth</code>（有源人脸核验，服务标识 <code>fv_auth</code>）</p>
    <p><span class="method post">POST</span><code>/v1/fv/self</code>（无源人脸识别，服务标识 <code>fv_self</code>）</p>
    <p>
      发起一次人脸核验，需 API 密钥权限覆盖 <code>fv_auth</code>（有源）或 <code>fv_self</code>（无源），
      或权限为 <code>all</code>。
      有源（auth）需传入 <code>name</code> 与 <code>id_card</code>（人脸 + 公安库比对）；
      无源（self）仅活体识别，无需姓名与证件号。两者均需 <code>notify_url</code>；
      <code>return_url</code> 可选，留空时核验完成后停留在平台认证结果页、不回跳下游，
      计费与资源包按子产品隔离（<code>fv_auth</code> 与 <code>fv_self</code> 各自独立资源包）。
    </p>
    <p>
      <strong>扣费时机：上游返回 token（接受本次核验）成功后才扣费</strong>——被上游拦截、连接失败时<strong>不扣费</strong>，
      订单仍会保留并写入失败原因（状态「超时/连接超时」），便于对账排查。
      发起前平台会校验额度（命中资源包或余额足够），不足时直接返回错误、不产生订单。
      命中资源包时按次扣减 1 次（订单「资源包扣减」列可见），未命中则扣余额；结果为不计费（6000/6100）时自动原路退还。
    </p>
    <h4>请求参数</h4>
    <table>
      <thead>
        <tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr>
      </thead>
      <tbody>
        <tr><td><code>name</code></td><td>string</td><td>有源必填</td><td>真实姓名</td></tr>
        <tr><td><code>id_card</code></td><td>string</td><td>有源必填</td><td>身份证号</td></tr>
        <tr><td><code>return_url</code></td><td>string</td><td>否</td><td>核身完成并校对后，用户浏览器最终跳转地址；留空时核验完成后停留平台认证结果页</td></tr>
        <tr><td><code>notify_url</code></td><td>string</td><td>是</td><td>核身结果异步通知地址</td></tr>
        <tr><td><code>biz_extra_data</code></td><td>string</td><td>否</td><td>业务扩展数据（原样回传）</td></tr>
      </tbody>
    </table>
    <h4>响应示例</h4>
    <pre><code>{
  "code": 0,
  "message": "success",
  "data": {
    "biz_no": "46382671905182934716",
    "site_url": "https://service.starloft.cn/fv/auth?token=xxx"
  }
}</code></pre>
    <p>
      <code>data.site_url</code> 为平台自站承接链接（PC 打开该链接会展示上游核身二维码供手机扫码，移动端打开会直接跳转上游核身页）。
      用户核身后平台先回跳 <code>/v1/fv/return</code> 完成一次结果校对，再 302 到你填写的 <code>return_url</code>，无需你自建中转页；
      未填写 <code>return_url</code> 时，承接页停留在认证结果界面（认证成功/失败）不做跳转。
    </p>

    <h3 id="fv-result">4.6 查询核验结果（FV）</h3>
    <p><span class="method post">POST</span><code>/v1/fv/result</code></p>
    <p>
      按业务流水号 <code>biz_no</code> 查询认证订单状态，用于前台轮询与结果校对，
      返回的是平台已落地的订单状态。<strong>建议优先配置 notify_url</strong>：
      结果由异步通知驱动落地，无需轮询即可实时到达。
    </p>
    <p>
      未配置 <code>notify_url</code> 的订单（「跳过异步通知」模式）没有任何推送，平台会在你轮询时
      按<strong>每笔订单的反查次数上限与最小间隔</strong>向上游同步结果并落地后再返回（上游对同一笔核身
      仅允许有限次结果反查，超出后该笔结果将不可取回）。因此请勿按秒级高频轮询：
      高频轮询不会让结果更早出现，只会增加无效请求，建议间隔 ≥ 5~10 秒并设置轮询上限
      （核身链接有效期约 15 分钟，过期未完成的订单平台会自动终结并退还已扣费用）。
    </p>
    <h4>请求参数</h4>
    <table>
      <thead>
        <tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr>
      </thead>
      <tbody>
        <tr><td><code>biz_no</code></td><td>string</td><td>是</td><td>发起人脸核验时返回的业务流水号</td></tr>
        <tr><td><code>sync</code></td><td>int</td><td>否</td><td>传 <code>1</code> 表示「立即校对一次」（如用户点击手动查询按钮），平台跳过最小间隔立即向上游同步一次；仍受每笔核身的反查次数上限约束</td></tr>
      </tbody>
    </table>
    <h4>响应示例</h4>
    <pre><code>{
  "code": 0,
  "message": "success",
  "data": {
    "biz_no": "46382671905182934716",
    "status": 2,
    "result_code": "1000",
    "result_message": "SUCCESS"
  }
}</code></pre>
    <p>
      <code>status</code>：<code>0</code> 待认证 / <code>1</code> 认证中 / <code>2</code> 认证成功 / <code>3</code> 认证失败 /
      <code>5</code> 超时结束（核身链接过期未完成、或上游结果已不可取回，不计费、已扣费用已退还）/ <code>6</code> 发起失败（上游拦截或连接超时，未扣费）；
      <code>status ≥ 2</code> 即为终态，可作为轮询结束条件。<code>result_code</code> / <code>result_message</code> 见「5.1 result_code 说明」。
    </p>

    <h3 id="fv-best-img">4.7 获取活体最佳图（FV）</h3>
    <p><span class="method post">POST</span><code>/v1/fv/best-img</code></p>
    <p>
      认证成功（<code>status=2</code>）后 <strong>24 小时内</strong>可领取该笔订单的活体最佳图（base64），
      <strong>每笔订单仅可领取一次</strong>；平台实时向上游取图转交，本地不落库。
    </p>
    <h4>请求参数</h4>
    <table>
      <thead>
        <tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr>
      </thead>
      <tbody>
        <tr><td><code>biz_no</code></td><td>string</td><td>是</td><td>发起人脸核验时返回的业务流水号</td></tr>
      </tbody>
    </table>
    <h4>响应示例</h4>
    <pre><code>{
  "code": 0,
  "message": "success",
  "data": {
    "biz_no": "46382671905182934716",
    "best_img": "data:image/jpeg;base64,xxxx"
  }
}</code></pre>

    <h3 id="fv-media">4.7.1 获取认证媒体（FV）</h3>
    <p><span class="method post">POST</span><code>/v1/fv/media</code></p>
    <p>
      认证成功（<code>status=2</code>）后，平台自动从上游下载该笔订单的活体照片（<code>image_best</code>）与
      验证视频（<code>video</code>），本地保存 <strong>30 天</strong>；期间可随时经本接口下载（均返回 base64），
      过期后自动清理不可再获取。
    </p>
    <h4>请求参数</h4>
    <table>
      <thead>
        <tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr>
      </thead>
      <tbody>
        <tr><td><code>biz_no</code></td><td>string</td><td>是</td><td>发起人脸核验时返回的业务流水号</td></tr>
      </tbody>
    </table>
    <h4>响应示例</h4>
    <pre><code>{
  "code": 0,
  "message": "success",
  "data": {
    "biz_no": "46382671905182934716",
    "media": {
      "image_best": "/9j/4AAQSkZJRg...（base64）",
      "video": "AAAAIGZ0eXBpc29t...（base64，无则缺省）"
    }
  }
}</code></pre>
    <p>
      视频为完整可播放的 MP4（base64），解密后存储，无需再向平台申请；若上游未返回某类媒体，对应字段缺省。
    </p>

    </template>

    <template v-if="isSms">
    <h3 id="sms-sign">4.8 创建短信签名（sms）</h3>
    <p><span class="method post">POST</span><code>/v1/sms/signs</code>（服务标识 <code>sms_sign</code>）</p>
    <p>
      提交短信签名报备（平台统一以「他公司」主体代下游客户报备）。需 API 密钥权限覆盖 <code>sms_sign</code>（或 <code>all</code>），
      账户完成个人实名（<code>realname_status ≥ 1</code>）；签名提交不收费。
    </p>
    <p>
      <strong>资质材料以图片 URL 形式传入：平台不做图片上传、转存或压缩，原样透传给上游报备。</strong>
      因此图片需你自行上传到可公网访问的地址（HTTPS、可直接下载，上游会主动拉取）。
    </p>
    <h4>请求参数</h4>
    <table>
      <thead>
        <tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr>
      </thead>
      <tbody>
        <tr><td><code>sign_name</code></td><td>string</td><td>是</td><td>签名内容，仅中文/英文/数字与圆括号（自动去【】、中文括号转英文）</td></tr>
        <tr><td><code>label</code></td><td>int</td><td>否</td><td>资质类型：1-营业执照（默认） 2-商标</td></tr>
        <tr><td><code>credit_code_url</code></td><td>string</td><td>是</td><td>营业执照图片 URL</td></tr>
        <tr><td><code>id_card_front</code> / <code>id_card_back</code></td><td>string</td><td>是</td><td>经办人身份证正面 / 反面图片 URL</td></tr>
        <tr><td><code>company</code></td><td>string</td><td>是</td><td>公司名称（实名主体）</td></tr>
        <tr><td><code>legal_person</code></td><td>string</td><td>是</td><td>法人姓名</td></tr>
        <tr><td><code>credit_code</code></td><td>string</td><td>是</td><td>统一社会信用代码</td></tr>
        <tr><td><code>credit_user_name</code></td><td>string</td><td>是</td><td>经办人姓名</td></tr>
        <tr><td><code>id_card</code></td><td>string</td><td>是</td><td>经办人身份证号</td></tr>
        <tr><td><code>phone</code></td><td>string</td><td>是</td><td>经办人手机号</td></tr>
        <tr><td><code>auth_letter</code></td><td>string</td><td>是</td><td>《短信签名授权书》图片 URL（加盖公章/签章）</td></tr>
        <tr><td><code>sx_commits</code></td><td>string</td><td>否</td><td>《用户接收意愿承诺函》图片 URL；未传时上游以营业执照兜底</td></tr>
        <tr><td><code>screenshot</code></td><td>string</td><td>否</td><td>商标备案截图 URL（<code>label=2</code> 时必填）</td></tr>
      </tbody>
    </table>
    <h4>响应示例</h4>
    <pre><code>{
  "code": 0,
  "message": "success",
  "data": {
    "sign_id": 12,
    "up_sign_id": "360974",
    "status": 0
  }
}</code></pre>
    <p>
      <code>sign_id</code> 为平台签名记录 ID，<code>up_sign_id</code> 为上游签名 ID，<code>status</code> 为平台审核状态（0-待审核 1-审核中 2-通过 3-驳回）。
      签名审核通过后可创建模板（<code>POST /v1/sms/templates</code>，以 <code>sign_name</code> 指定绑定签名）并发送短信。
    </p>

    <h3 id="sms-sign-get">4.8.1 查询与删除短信签名（sms）</h3>
    <p><span class="method get">GET</span><code>/v1/sms/signs/:id</code>（服务标识 <code>sms_sign_get</code>）</p>
    <p>
      查询某个签名的审核状态。<code>:id</code> 支持平台签名记录 ID（创建接口返回的 <code>sign_id</code>）或上游签名 ID（<code>up_sign_id</code>），
      仅能查询本账号的签名。需 API 密钥权限覆盖 <code>sms_sign_get</code>（或 <code>all</code>）。
    </p>
    <h4>响应示例</h4>
    <pre><code>{
  "code": 0,
  "message": "success",
  "data": {
    "sign": {
      "sign_id": 12,
      "up_sign_id": "360974",
      "status": 2,
      "sign_name": "示例科技",
      "result_message": ""
    }
  }
}</code></pre>
    <p><span class="method delete">DELETE</span><code>/v1/sms/signs/:id</code>（服务标识 <code>sms_sign_delete</code>）</p>
    <p>
      删除签名：平台先删除上游签名，成功后再删除本地记录（删除失败即中止，避免本地与上游不一致）。
      <code>:id</code> 取值同查询接口；需 API 密钥权限覆盖 <code>sms_sign_delete</code>（或 <code>all</code>）。
    </p>
    <h4>响应示例</h4>
    <pre><code>{ "code": 0, "message": "success" }</code></pre>
    <p>
      除主动查询外，平台还会在签名/模板审核状态变更时向下游推送（后台「系统设置 → 短信服务」配置回调地址 <code>SMS_STATUS_NOTIFY_URL</code>）：
      请求体为 <code>{"biz_type":"sign|template","record_id":12,"up_id":"360974","status":2,"reason":"","sign":"..."}</code>，
      签名算法为对 <code>biz_type / reason / record_id / status / up_id</code> 按字段名字典序拼接后取 HMAC-SHA256（小写十六进制），
      密钥为该账号主 API 密钥的 <code>api_secret</code>；接收方校验通过后应返回 2xx，否则平台按指数退避补推。
    </p>

    <h3 id="sms-send">4.9 短信发送（sms）</h3>
    <p><span class="method post">POST</span><code>/v1/sms/send</code>（服务标识 <code>sms_send</code>）</p>
    <p>
      需 API 密钥权限覆盖 <code>sms_send</code>（或 <code>all</code>），且账户完成个人实名（<code>realname_status=1</code>）即可。
      本接口将发送请求透传联麓（shlianlu）短信网关，<strong>仅支持模板发送</strong>（验证码 / 通知 / 营销场景）；
      成功后按短信资源包扣除费用——按所发<strong>模板的类型</strong>匹配资源包（营销模板扣营销资源包，验证码/通知模板扣通用短信资源包，两类互不通用），余额兜底。
    </p>
    <p>
      <strong>计费按短信条数（长短信分条）。</strong>国内短信一条按 70 字符计费（含标点与空格，签名与正文一并计入）；
      超过 70 字符按每 67 字符递增 1 条（71-134 字 2 条、135-201 字 3 条……），最多 1000 字符 / 15 条。
      计费条数 = 号码数 × 单条分条数，实际以上游返回的预扣费条数为准。
      <strong>扣费时机：上游返回成功后才扣费</strong>——被上游拦截的失败不扣费；命中短信资源包时按条扣减（发送记录「扣费」列显示扣减条数），未命中则扣余额。
      若后续回执失败（未送达），平台会自动退还本次扣费（资源包退回条数 / 余额原路退还）。
      发送前平台会校验额度（资源包剩余条数或余额是否足够），不足时直接拒绝发送、不产生发送记录；发送失败仍会保留记录并写入失败原因。
    </p>
    <p>
      <strong>签名与模板须先报备审核。</strong>使用本接口前，请在星楼网络控制台「短信签名 / 短信模板」页面在线提交
      签名与模板，经联麓审核通过后方可发送：签名按「他公司」主体报备，需提供企业名称、统一社会信用代码、法人/经办人信息与营业执照
      （非营销短信未上传《意愿承诺函》时自动使用营业执照图片），发送时通过 <code>sign_name</code> 选择审核通过的签名，
      <code>template_id</code> 与 <code>template_params</code> 选择审核通过的模板并填充参数。
      模板报备时须绑定一个已审核通过的签名（控制台「短信模板—关联签名」选择，或创建模板接口的 <code>sign_name</code> 指定）。
      绑定<strong>平台公共签名</strong>的模板需先经平台审核，通过后才报备上游（审核期间模板状态为待平台审核，暂不可发送）；绑定自有签名的模板提交即报备上游。
      平台会校验<strong>归属</strong>：模板与 <code>sign_name</code> 均须属于当前账号且已审核通过，否则拒绝发送。
    </p>
    <h4>请求参数</h4>
    <table>
      <thead>
        <tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr>
      </thead>
      <tbody>
        <tr><td><code>phone_number_set</code></td><td>string[]</td><td>是</td><td>接收手机号列表</td></tr>
        <tr><td><code>sign_name</code></td><td>string</td><td>是</td><td>短信签名</td></tr>
        <tr><td><code>template_id</code></td><td>string</td><td>是</td><td>短信模板 ID</td></tr>
        <tr><td><code>template_params</code></td><td>string[]</td><td>否</td><td>模板参数（顺序与模板占位符一致）</td></tr>
        <tr><td><code>sms_type</code></td><td>string</td><td>否</td><td>短信类型：验证码 / 通知 / 营销；仅作兼容字段，实际按所发模板的类型计费与扣包</td></tr>
        <tr><td><code>session_context</code></td><td>string</td><td>否</td><td>会话上下文（原样回传）</td></tr>
        <tr><td><code>notify_url</code></td><td>string</td><td>否</td><td>回执主动推送地址：上游回执到达后，平台将主动 POST 通知该地址（携带 HMAC 签名），详见「4.10 回执主动推送」</td></tr>
      </tbody>
    </table>
    <h4>请求示例</h4>
    <pre><code>{
  "phone_number_set": ["13800138000"],
  "sign_name": "星楼网络",
  "template_id": "1001",
  "template_params": ["123456", "5"],
  "sms_type": "notify",
  "notify_url": "https://yourdomain.com/sms/receipt"
}</code></pre>

    <h3 id="sms-report">4.10 查询发送回执（sms）</h3>
    <p><span class="method post">POST</span><code>/v1/sms/report</code></p>
    <p>
      按发送接口返回的 <code>message_sid</code>（即上游任务ID）实时查询该批短信的回执明细。
      平台将请求透传联麓（shlianlu）拉取报告接口并原样返回，便于下游感知每台手机号的具体送达状态
      （如号码失效、黑名单、签名/模板未通过等）。
    </p>
    <h4>请求参数</h4>
    <table>
      <thead>
        <tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr>
      </thead>
      <tbody>
        <tr><td><code>task_id</code></td><td>string</td><td>是</td><td>发送接口返回的 <code>message_sid</code> / <code>request_id</code>（上游任务ID）</td></tr>
        <tr><td><code>page_no</code></td><td>int</td><td>否</td><td>页码，从 1 开始</td></tr>
        <tr><td><code>page_size</code></td><td>int</td><td>否</td><td>每页条数，默认 10</td></tr>
      </tbody>
    </table>
    <h4>响应示例</h4>
    <pre><code>{
  "code": 0,
  "message": "success",
  "data": {
    "task_id": "202203170025179000005",
    "reports": [
      {
        "sequenceId": "100941727",
        "phone": "15601862749",
        "content": "【星楼网络】您正在验证，验证码123456...",
        "status": "1",
        "respTime": "2026-09-18 17:10:20",
        "respCode": "DELIVRD",
        "codeDesc": "发送成功"
      }
    ]
  }
}</code></pre>
    <p>
      <code>reports[].respCode</code> 为 <code>DELIVRD</code> 表示该号码发送成功，其余为失败（见短信状态码表）。
      平台同时在 <code>/v1/callback/sms-report</code> 提供短信回执推送（联麓「发送状态推送地址」指向该地址），
      可按 <code>taskId</code> 回溯并落库回执；两者配合可实现异步回执与主动查询。
    </p>

    <h3 id="sms-notify">4.11 回执主动推送（sms）</h3>
    <p>
      发送短信时若提供了 <code>notify_url</code>，平台在收到上游（联麓）逐条回执推送后，
      会以 <strong>POST</strong> 方式（<code>Content-Type: application/json</code>）主动转发到你指定的
      <code>notify_url</code>，无需再主动拉取。每一条号码回执推送一次。
    </p>
    <h4>请求体（平台 → 下游）</h4>
    <pre><code>{
  "biz_no": "46382671905182934716",
  "message_sid": "202203170025179000005",
  "phone": "13800138000",
  "status": "success",
  "resp_code": "DELIVRD",
  "code_desc": "发送成功",
  "resp_time": "2026-09-18 17:10:20",
  "sequence_id": "100941727",
  "fail_message": "",
  "sign": "a1b2c3d4e5f6..."
}</code></pre>
    <h4>字段说明</h4>
    <table>
      <thead>
        <tr><th>字段</th><th>类型</th><th>说明</th></tr>
      </thead>
      <tbody>
        <tr><td><code>biz_no</code></td><td>string</td><td>平台发送记录号（发送接口 <code>request_id</code>）</td></tr>
        <tr><td><code>message_sid</code></td><td>string</td><td>上游任务 ID（发送接口返回）</td></tr>
        <tr><td><code>phone</code></td><td>string</td><td>本次回执对应的接收号码</td></tr>
        <tr><td><code>status</code></td><td>string</td><td>本号码送达状态：<code>success</code> 成功 / <code>fail</code> 失败</td></tr>
        <tr><td><code>resp_code</code></td><td>string</td><td>上游回执码，<code>DELIVRD</code> 为成功</td></tr>
        <tr><td><code>code_desc</code></td><td>string</td><td>回执码中文描述</td></tr>
        <tr><td><code>resp_time</code></td><td>string</td><td>回执时间（状态未知时为空）</td></tr>
        <tr><td><code>sequence_id</code></td><td>string</td><td>上游回执序列 ID（唯一）</td></tr>
        <tr><td><code>fail_message</code></td><td>string</td><td>失败原因（成功时为空）</td></tr>
        <tr><td><code>sign</code></td><td>string</td><td>HMAC-SHA256 签名，基于你的 API Secret，供校验防伪造</td></tr>
      </tbody>
    </table>
    <h4>签名校验</h4>
    <p>
      取 <code>biz_no</code>、<code>code_desc</code>、<code>message_sid</code>、<code>phone</code>、
      <code>resp_code</code>、<code>status</code> 六个字段，按 key 字典序拼接为
      <code>k=v&amp;k=v...</code>（不做 URL 编码），再以
      <code>HMAC-SHA256(api_secret, canonical)</code> 计算十六进制小写签名：
    </p>
    <pre><code>canonical = "biz_no=xxx&amp;code_desc=xxx&amp;message_sid=xxx&amp;phone=xxx&amp;resp_code=xxx&amp;status=xxx"
sign = hex(HMAC-SHA256(api_secret, canonical))</code></pre>
    <p>校验失败应拒绝该通知（返回非 2xx）。若通知丢失，可调用 <code>/v1/sms/report</code> 按 <code>message_sid</code> 主动查询。</p>

    <h3 id="sms-reply">4.12 短信回复推送与查询（sms）</h3>
    <p>
      发送短信后，若接收号码回复短信，上游（联麓）会推送回复给平台；平台会
      <strong>主动 POST</strong> 到发送时配置的 <code>notify_url</code>。同时提供主动查询接口
      <code>/v1/sms/replies</code>：提供 <code>date</code> 时平台先按日向上游拉取回复并落库，再返回本用户回复记录。
    </p>
    <h4>回执式主动推送（平台 → 下游 notify_url）</h4>
    <pre><code>{
  "biz_no": "46382671905182934716",
  "message_sid": "202203170025179000005",
  "phone": "13800138000",
  "content_down": "【星楼网络】您的验证码是123456",
  "content_up": "T",
  "sequence_id": "107067357",
  "timestamp": "1647485562153",
  "status": "00",
  "tag": "",
  "sign": "a1b2c3d4e5f6..."
}</code></pre>
    <h4>签名校验</h4>
    <p>
      取 <code>biz_no</code>、<code>content_down</code>、<code>content_up</code>、<code>message_sid</code>、
      <code>phone</code>、<code>status</code> 六个字段，按 key 字典序拼接后以
      <code>HMAC-SHA256(api_secret, canonical)</code> 计算十六进制小写签名（与 4.10 同算法），校验失败应拒绝该通知。
    </p>
    <h4>主动查询</h4>
    <p><span class="method post">POST</span><code>/v1/sms/replies</code></p>
    <table>
      <thead>
        <tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr>
      </thead>
      <tbody>
        <tr><td><code>date</code></td><td>string</td><td>否</td><td>回复日期（yyyyMMdd）；提供时先向上游拉取该日回复并落库</td></tr>
        <tr><td><code>task_id</code></td><td>string</td><td>否</td><td>发送接口返回的 <code>message_sid</code>，按任务过滤</td></tr>
        <tr><td><code>page_no</code></td><td>int</td><td>否</td><td>页码，从 1 开始</td></tr>
        <tr><td><code>page_size</code></td><td>int</td><td>否</td><td>每页条数，默认 10</td></tr>
      </tbody>
    </table>
    <h4>响应示例</h4>
    <pre><code>{
  "code": 0,
  "message": "success",
  "data": {
    "total": 1,
    "replies": [
      {
        "task_id": "202203170025179000005",
        "phone": "13800138000",
        "sequence_id": "107067357",
        "content_down": "【星楼网络】您的验证码是123456",
        "content_up": "T",
        "resp_time": "1647485562153",
        "status": "00",
        "tag": ""
      }
    ]
  }
}</code></pre>
    <p>
      上游已拉取的回复会标记「已读」，再次拉取不再返回；平台拉取后落库本地（去重），
      后续按 <code>date</code> / <code>task_id</code> 查询由本地兜底，不依赖上游重复拉取。
    </p>

    </template>

    <template v-if="isFv">
    <h2>5. 回调说明（notify_url 与 return_url）</h2>
    <p>
      发起人脸核验时需提供 <code>notify_url</code>（异步通知），<code>return_url</code>（同步跳转）可选，
      用于认证结果回流。两者触发时机与数据格式如下。
    </p>

    <h3>5.1 异步通知（notify_url）</h3>
    <p>
      认证产生最终结果（成功或失败）后，平台会以 <strong>POST</strong> 方式向你的
      <code>notify_url</code> 发送通知，<code>Content-Type: application/json</code>。
      通知携带 <code>sign</code>（HMAC-SHA256 签名，基于你的 API Secret 生成），
      下游应校验签名后再落地结果，防止伪造回调；同时可依据 <code>biz_no</code>（平台订单号）关联订单。
    </p>

    <h4>请求体</h4>
    <pre><code>{
  "biz_no": "46382671905182934716",
  "status": 2,
  "result_code": "1000",
  "result_message": "SUCCESS",
  "cost": 1.50,
  "sign": "a1b2c3d4e5f6..."
}</code></pre>

    <h4>字段说明</h4>
    <table>
      <thead>
        <tr>
          <th>字段</th>
          <th>类型</th>
          <th>说明</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td><code>biz_no</code></td>
          <td>string</td>
          <td>平台订单号（创建认证订单时返回，用于关联订单）</td>
        </tr>
        <tr>
          <td><code>status</code></td>
          <td>int</td>
          <td>订单状态：<code>2</code> 认证成功 / <code>3</code> 认证失败 / <code>5</code> 超时结束（不计费已退款）/ <code>6</code> 发起失败（未扣费）</td>
        </tr>
        <tr>
          <td><code>result_code</code></td>
          <td>string</td>
          <td>上游认证结果码，见下方「result_code 说明」</td>
        </tr>
        <tr>
          <td><code>result_message</code></td>
          <td>string</td>
          <td>结果说明（英文常量，如 SUCCESS）</td>
        </tr>
        <tr>
          <td><code>cost</code></td>
          <td>number</td>
          <td>本次认证扣费金额（元）</td>
        </tr>
        <tr>
          <td><code>sign</code></td>
          <td>string</td>
          <td>回调签名，用于防止伪造回调（算法见下方「签名算法」）</td>
        </tr>
      </tbody>
    </table>

    <h4>签名算法</h4>
    <p>
      取 <code>biz_no</code>、<code>cost</code>、<code>result_code</code>、
      <code>result_message</code>、<code>status</code> 五个字段，
      按 key 字典序拼接为原始字符串（不做 URL 编码），以 API Secret 计算 HMAC-SHA256：
    </p>
    <pre><code>canonical = "biz_no=xxx&amp;cost=1.50&amp;result_code=1000&amp;result_message=xxx&amp;status=2"
sign = 小写hex( HMAC-SHA256(api_secret, canonical) )</code></pre>
    <p>其中 <code>cost</code> 固定保留两位小数（如 <code>1.50</code>），<code>status</code> 为整数。校验失败应拒绝该通知（返回非 2xx）。若通知丢失，可调用 <code>/v1/fv/result</code> 按 <code>biz_no</code> 主动查询。</p>

    <h4>result_code 说明</h4>
    <table>
      <thead>
        <tr>
          <th>result_code</th>
          <th>result_message</th>
          <th>含义</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td><code>1000</code></td>
          <td>SUCCESS</td>
          <td>认证通过</td>
        </tr>
        <tr>
          <td><code>2000</code></td>
          <td>PASS_LIVING_NOT_THE_SAME</td>
          <td>活体通过，但与身份证非同一人</td>
        </tr>
        <tr>
          <td><code>3000</code></td>
          <td>NO_ID_CARD_NUMBER / ID_NUMBER_NAME_NOT_MATCH / NO_FACE_FOUND / NO_ID_PHOTO / PHOTO_FORMAT_ERROR</td>
          <td>证号、姓名不匹配或未检出人脸等（认证不通过）</td>
        </tr>
        <tr>
          <td><code>3000</code></td>
          <td>DATA_SOURCE_ERROR / INTERNAL_ERROR</td>
          <td>数据源或服务器临时异常</td>
        </tr>
        <tr>
          <td><code>4000</code></td>
          <td>FAIL_LIVING_FACE_ATTACK</td>
          <td>活体攻击 / 活体检测失败</td>
        </tr>
        <tr>
          <td><code>6000</code></td>
          <td>FAILED / CANCELLED / TIMEOUT</td>
          <td>流程异常结束 / 用户取消 / 超时</td>
        </tr>
        <tr>
          <td><code>6100</code></td>
          <td>SUPPORT_ERROR / PERMISSIONS_ERROR / OTHER_ERROR</td>
          <td>webRTC / 摄像头权限等问题</td>
        </tr>
      </tbody>
    </table>
    <p>
      <code>6000</code> / <code>6100</code> 为不计费结果，平台会自动退还预扣金额；
      <code>NOT_STARTED</code> / <code>PROCESSING</code> 表示认证尚未完结，不会作为最终通知发送。
    </p>
    <p>核身未产生上游结果时，平台以自有结果码结束订单（均为不计费，已扣费用自动退还）：</p>
    <table>
      <thead>
        <tr>
          <th>result_code</th>
          <th>对应 status</th>
          <th>含义</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td><code>TIMEOUT</code></td>
          <td><code>6</code></td>
          <td>发起阶段上游拦截 / 连接超时，未产生扣费</td>
        </tr>
        <tr>
          <td><code>EXPIRED</code></td>
          <td><code>5</code></td>
          <td>核身链接超时未完成，不计费并退还已扣费用</td>
        </tr>
        <tr>
          <td><code>DATA_DESTROYED</code></td>
          <td><code>5</code></td>
          <td>上游结果已不可取回（上游结果反查每笔仅允许 3 次调用，超出次数或超过 1 天有效期即销毁），视为未完成核身，不计费并退还已扣费用（不代表核身失败）</td>
        </tr>
      </tbody>
    </table>
    <p>
      上述结果同样经 <code>notify_url</code> 推送，下游应以「通知为准」并配合低频轮询，请勿按秒级高频轮询
      <code>/v1/fv/result</code>：平台内部向上游反查结果的次数每笔订单有限（且平台已按额度预算控制），
      高频轮询不会让结果更早出现，只会增加无效请求。
    </p>

    <h4>响应要求</h4>
    <p>下游收到通知后返回 HTTP 200～299 即可（平台校验 2xx 视为通知成功，否则后续重试）。</p>

    <h3>5.2 同步跳转（return_url）</h3>
    <p>
      认证结果确定后，平台会将用户浏览器以 <strong>GET</strong> 方式 302 跳转回你发起认证时填写的
      <code>return_url</code>，且<strong>不附加任何参数</strong>（跳转地址与你传入的 return_url 完全一致，
      认证失败同样跳回该 return_url）。
      下游应依据自己记录的 <code>biz_no</code> 展示结果，最终核验状态以异步通知（5.1）为准。
    </p>
    <p>示例：</p>
    <pre><code>https://yourdomain.com/certification/star_loft_fv_for_zjmf_mfcw/result</code></pre>
    <p>
      未填写 <code>return_url</code>（留空）时，平台不发起回跳：用户核身后停留在平台承接页的认证结果界面
      （认证成功展示「认证成功」并停留；认证失败展示失败原因），结果仍以异步通知（5.1）为准。
      填写 <code>return_url</code> 时，承接页会在认证结束后展示结果并倒计时 3 秒自动跳转到该地址（也可点击「立即返回」）。
    </p>
    <p>
      说明：<code>return_url</code> 用于用户侧页面回流，<code>notify_url</code> 用于服务端结果落地，
      两者配合使用；若 <code>notify_url</code> 未及时到达，可调用 <code>/v1/fv/result</code> 按 <code>biz_no</code> 主动查询。
    </p>

    </template>

    <template v-if="isSms">
    <h2>6. 调用示例（curl）</h2>
    <p>以短信发送为例：</p>
    <pre><code>BODY='{"phone_number_set":["13800138000"],"sign_name":"星楼网络","template_id":"1001","template_params":["123456","5"]}'
SIGN=$(printf '%s' "$BODY" | openssl dgst -sha256 -hmac "your_api_secret" | awk '{print $2}')
TS=$(date +%s)

curl -X POST "https://api.starloft.cn/v1/sms/send" \
  -H "Content-Type: application/json" \
  -H "X-Api-Key: your_api_key" \
  -H "X-Sign: $SIGN" \
  -H "X-Sign-Version: hmac_sha256" \
  -H "X-Timestamp: $TS" \
  -d "$BODY"</code></pre>
    </template>

    <h2>7. 错误码</h2>
    <table>
      <thead>
        <tr>
          <th>code</th>
          <th>说明</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td><code>0</code></td>
          <td>成功</td>
        </tr>
        <tr>
          <td><code>400</code></td>
          <td>参数错误 / 余额不足</td>
        </tr>
        <tr>
          <td><code>401</code></td>
          <td>鉴权失败（API Key 无效、签名错误或时间戳过期）</td>
        </tr>
        <tr>
          <td><code>403</code></td>
          <td>权限不足（用户被禁用、实名等级不满足该接口要求、密钥权限未覆盖该接口）</td>
        </tr>
        <tr>
          <td><code>404</code></td>
          <td>订单不存在</td>
        </tr>
        <tr>
          <td><code>500</code></td>
          <td>系统内部错误</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

// API v1 接口文档：/{产品}/api/v1（接口按产品过滤展示）
const route = useRoute()
const product = computed(() => {
  const p = route.path.split('/')[2]
  return ['fv', 'sms'].includes(p) ? p : 'fv'
})
const isFv = computed(() => product.value === 'fv')
const isSms = computed(() => product.value === 'sms')
</script>