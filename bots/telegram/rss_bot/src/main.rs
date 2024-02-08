use std::time::Duration;
use teloxide::{prelude::*, utils::command::BotCommands};
extern crate pretty_env_logger;
#[macro_use]
extern crate log;

#[derive(Default)]
pub struct Feed {
    pub id: i64,
    pub feed_link: String,
    pub link: String,
    pub title: String,
    pub description: String,
}

#[derive(Default, PartialEq)]
pub struct Item {
    pub feed_id: i64,
    pub link: String,
    pub title: String,
    pub description: String,
}

impl Feed {
    pub fn new(
        id: Option<i64>,
        feed_link: String,
        link: String,
        title: String,
        description: String,
    ) -> Self {
        Self {
            id: id.unwrap_or(0),
            feed_link,
            link,
            title,
            description,
        }
    }

    pub fn from_rss(channel: &rss::Channel, feed_link: String) -> Self {
        Self {
            id: 0,
            feed_link,
            link: channel.link().to_string(),
            title: channel.title().to_string(),
            description: channel.description().to_string(),
        }
    }

    pub fn to_string(&self) -> String {
        format!("{}\n{}\n{}", self.title, self.description, self.link,)
    }
}

impl Item {
    pub fn new(feed_id: i64, link: String, title: String, description: String) -> Self {
        Self {
            feed_id,
            link,
            title,
            description,
        }
    }

    pub fn from_rss_with_feed_id(feed_id: i64, item: &rss::Item) -> Self {
        Self {
            feed_id,
            link: item.link().unwrap().to_string(),
            title: item.title().unwrap().to_string(),
            description: item.description().unwrap().to_string(),
        }
    }
}

fn init_db() -> Result<(), Box<dyn std::error::Error>> {
    let conn = sqlite::open("rss.sqlite")?;
    conn.execute(
        "CREATE TABLE IF NOT EXISTS feed_tab (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        feed_link TEXT NOT NULL,
        link TEXT NOT NULL,
        title TEXT NOT NULL,
        description TEXT NOT NULL)",
    )?;

    conn.execute(
        "CREATE TABLE IF NOT EXISTS item_tab (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        feed_id INTEGER NOT NULL,
        title TEXT NOT NULL,
        link TEXT NOT NULL,
        description TEXT NOT NULL)",
    )?;
    Ok(())
}

async fn get_rss(url: &str) -> Result<rss::Channel, Box<dyn std::error::Error>> {
    let content = reqwest::get(url).await?.bytes().await?;
    let channel = rss::Channel::read_from(&content[..])?;
    Ok(channel)
}

fn save_feed(feed: &Feed) -> Result<i64, Box<dyn std::error::Error>> {
    info!(
        "save feed info: {}, {}, {}",
        feed.link, feed.title, feed.description
    );
    let conn = sqlite::open("rss.sqlite")?;

    if let Ok(mut stmt) = conn.prepare("SELECT * FROM feed_tab WHERE link = :link") {
        stmt.bind((":link", feed.link.as_str()))?;
        if let sqlite::State::Row = stmt.next()? {
            return Ok(0);
        }
    }
    let stmt = format!(
        "INSERT INTO feed_tab (feed_link, link, title, description) VALUES ('{}', '{}', '{}', '{}')",
        feed.feed_link.replace("'", "''"),
        feed.link.replace("'", "''"),
        feed.title.replace("'", "''"),
        feed.description.replace("'", "''")
    );
    conn.execute(stmt)?;

    let mut stmt = conn.prepare("SELECT last_insert_rowid()")?;
    if let sqlite::State::Row = stmt.next()? {
        let id = stmt.read::<i64, _>(0)?;
        return Ok(id);
    }

    Err("save feed failed".into())
}

fn list_subscribed() -> Result<Vec<Feed>, Box<dyn std::error::Error>> {
    let conn = sqlite::open("rss.sqlite")?;
    let mut stmt = conn.prepare("SELECT * FROM feed_tab")?;
    let mut feeds = Vec::new();
    while let sqlite::State::Row = stmt.next()? {
        let feed = Feed::new(
            Some(stmt.read::<i64, _>("id")?),
            stmt.read::<String, _>("feed_link")?,
            stmt.read::<String, _>("link")?,
            stmt.read::<String, _>("title")?,
            stmt.read::<String, _>("description")?,
        );
        feeds.push(feed);
    }
    Ok(feeds)
}

fn get_feed_link(feed_id: i64) -> Result<String, Box<dyn std::error::Error>> {
    let conn = sqlite::open("rss.sqlite")?;
    let mut stmt = conn.prepare("SELECT * FROM feed_tab WHERE id = :id")?;
    stmt.bind((":id", feed_id))?;
    if let sqlite::State::Done = stmt.next()? {
        return Err("feed not found".into());
    }
    let link = stmt.read::<String, _>("feed_link")?;
    Ok(link)
}

#[tokio::main]
async fn main() {
    init_db().unwrap();
    pretty_env_logger::init();
    info!("starting rss bot...");
    let bot = Bot::from_env();

    Command::repl(bot, action).await;
}

#[derive(BotCommands, Clone)]
#[command(
    rename_rule = "lowercase",
    description = "These commands are supported:"
)]
enum Command {
    #[command(description = "print help message. (will start backgroud trigger, every 10 minutes)")]
    Start,
    #[command(description = "subscribe a rss link, usage: /sub [link]")]
    Sub { link: String },
    #[command(description = "list subscribed rss, usage: /list")]
    List,
    #[command(description = "get rss items, usage: /get [index] [size]", parse_with = "split")]
    Get { feed_id: i64, num: usize },
}

async fn action(bot: Bot, msg: Message, cmd: Command) -> ResponseResult<()> {
    match cmd {
        Command::Start => {
            bot.send_message(msg.chat.id, Command::descriptions().to_string())
                .await?;

            tokio::spawn(async move {
                loop {
                    trigger(&bot, &msg).await.unwrap();
                    tokio::time::sleep(Duration::from_secs(600)).await;
                }
            });
        }
        Command::Sub { link } => {
            cmd_subscribe_new(bot, msg, link).await?;
        }
        Command::List => {
            cmd_list_subscribed(bot, msg).await?;
        }
        Command::Get { feed_id, num } => {
            cmd_get_feed_items(bot, msg, feed_id, num).await?;
        }
    }
    Ok(())
}

async fn cmd_subscribe_new(bot: Bot, msg: Message, link: String) -> ResponseResult<()> {
    info!("subscribe new rss: {}", link);
    let channel = get_rss(&link).await.unwrap();
    let feed = Feed::from_rss(&channel, link);
    let id = save_feed(&feed).unwrap();
    if id == 0 {
        bot.send_message(msg.chat.id, "already exists subscribe")
            .await?;
        return Ok(());
    }
    bot.send_message(msg.chat.id, "subscribe success").await?;
    Ok(())
}

async fn cmd_list_subscribed(bot: Bot, msg: Message) -> ResponseResult<()> {
    bot.send_message(msg.chat.id, "list subscribed ing...")
        .await?;
    let feeds = list_subscribed().unwrap();
    for feed in feeds.iter() {
        bot.send_message(msg.chat.id, feed.to_string()).await?;
    }
    bot.send_message(msg.chat.id, "list subscribed done")
        .await?;

    Ok(())
}

async fn trigger(bot: &Bot, msg: &Message) -> ResponseResult<()> {
    let feeds = list_subscribed().unwrap();
    for feed in feeds.iter() {
        let channel = get_rss(&feed.feed_link).await.unwrap();
        let items = channel.items;
        for upstream_item in items.iter() {
            let item = Item::from_rss_with_feed_id(feed.id, upstream_item);
            let db_item = search_item_with_title(&item.title).unwrap();
            if db_item != Item::default() {
                continue;
            }
            let str = format!(
                "You have new Update\n{}\n{}\n{}",
                feed.title, item.title, item.link
            );
            save_item(&item).unwrap();
            bot.send_message(msg.chat.id, str).await?;
        }
    }
    Ok(())
}

fn save_item(item: &Item) -> Result<(), Box<dyn std::error::Error>> {
    let conn = sqlite::open("rss.sqlite")?;
    let mut stmt =
        conn.prepare("INSERT INTO item_tab VALUES (NULL, :feed_id, :title, :link, :description)")?;
    stmt.bind((":feed_id", item.feed_id))?;
    stmt.bind((":title", item.title.as_str()))?;
    stmt.bind((":link", item.link.as_str()))?;
    stmt.bind((":description", item.description.as_str()))?;
    stmt.next()?;
    Ok(())
}

fn search_item_with_title(title: &str) -> Result<Item, Box<dyn std::error::Error>> {
    let conn = sqlite::open("rss.sqlite")?;
    let mut stmt = conn.prepare("SELECT * FROM item_tab WHERE title = :title")?;
    stmt.bind((":title", title))?;
    if let sqlite::State::Done = stmt.next()? {
        return Ok(Item::default());
    }
    let item = Item {
        feed_id: stmt.read::<i64, _>("feed_id")?,
        title: stmt.read::<String, _>("title")?,
        link: stmt.read::<String, _>("link")?,
        description: stmt.read::<String, _>("description")?,
    };

    Ok(item)
}

async fn cmd_get_feed_items(
    bot: Bot,
    msg: Message,
    feed_id: i64,
    size: usize,
) -> ResponseResult<()> {
    bot.send_message(msg.chat.id, "get rss items ing...")
        .await?;
    let link = get_feed_link(feed_id).unwrap();
    debug!("get rss link: {}", link);
    let channel = get_rss(&link).await.unwrap();
    for item in channel.items.iter().take(size) {
        // let item = Item::from_rss_with_feed_id(feed_id, item);
        let str = format!("{}\n{}", item.title().unwrap(), item.link().unwrap());
        bot.send_message(msg.chat.id, str).await?;
    }
    bot.send_message(msg.chat.id, "get rss items done").await?;

    Ok(())
}
